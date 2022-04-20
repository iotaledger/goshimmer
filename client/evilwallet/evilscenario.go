package evilwallet

import (
	"fmt"
	"strconv"

	"github.com/mr-tron/base58"
	"go.uber.org/atomic"

	"github.com/iotaledger/hive.go/types"
)

// The custom conflict in spammer can be provided like this:
// EvilBatch{
// 	{
// 		ScenarioAlias{inputs: []{"1"}, outputs: []{"2","3"}}
// 	},
// 	{
// 		ScenarioAlias{inputs: []{"2"}, outputs: []{"4"}},
// 		ScenarioAlias{inputs: []{"2"}, outputs: []{"5"}}
// 	}
// }

type ScenarioAlias struct {
	Inputs  []string
	Outputs []string
}

func NewScenarioAlias() ScenarioAlias {
	return ScenarioAlias{
		Inputs:  make([]string, 0),
		Outputs: make([]string, 0),
	}
}

type EvilBatch [][]ScenarioAlias

type EvilScenario struct {
	ID string
	// provides a user-friendly way of listing input and output aliases
	ConflictBatch EvilBatch
	// determines whether outputs of the batch  should be reused during the spam to create deep UTXO tree structure.
	Reuse bool
	// if provided, the outputs from the spam will be saved into this wallet, accepted types of wallet: Reuse, RestrictedReuse.
	// if type == Reuse, then wallet is available for reuse spamming scenarios that did not provide RestrictedWallet.
	OutputWallet *Wallet
	// if provided and reuse set to true, outputs from this wallet will be used for deep spamming, allows for controllable building of UTXO deep structures.
	// if not provided evil wallet will use Reuse wallet if any is available. Accepts only RestrictedReuse wallet type.
	RestrictedInputWallet *Wallet
	// used together with scenario ID to create a prefix for distinct batch alias creation
	BatchesCreated     *atomic.Uint64
	NumOfClientsNeeded int
}

func NewEvilScenario(options ...ScenarioOption) *EvilScenario {
	scenario := &EvilScenario{
		ConflictBatch:  SingleTransactionBatch(),
		Reuse:          false,
		OutputWallet:   NewWallet(),
		BatchesCreated: atomic.NewUint64(0),
	}

	for _, option := range options {
		option(scenario)
	}
	scenario.ID = base58.Encode([]byte(fmt.Sprintf("%v%v%v", scenario.ConflictBatch, scenario.Reuse, scenario.OutputWallet.ID)))[:11]
	scenario.NumOfClientsNeeded = calculateNumofClientsNeeded(scenario)
	return scenario
}

func calculateNumofClientsNeeded(scenario *EvilScenario) (counter int) {
	for _, conflictMap := range scenario.ConflictBatch {
		if len(conflictMap) > counter {
			counter = len(conflictMap)
		}
	}
	return
}

// readCustomConflictsPattern determines outputs of the batch, needed for saving batch outputs to the outputWallet.
func (e *EvilScenario) readCustomConflictsPattern(batch EvilBatch) (batchOutputs map[string]types.Empty) {
	outputs := make(map[string]types.Empty)
	inputs := make(map[string]types.Empty)

	for _, conflictMap := range batch {
		for _, conflicts := range conflictMap {
			// add output to outputsAliases
			for _, input := range conflicts.Inputs {
				inputs[input] = types.Void
			}
			for _, output := range conflicts.Outputs {
				outputs[output] = types.Void
			}
		}
	}
	// remove outputs that were never used as input in this EvilBatch to determine batch outputs
	for output := range outputs {
		if _, ok := inputs[output]; ok {
			delete(outputs, output)
		}
	}
	batchOutputs = outputs
	return
}

// NextBatchPrefix creates a new batch prefix by increasing the number of created batches for this scenario.
func (e *EvilScenario) nextBatchPrefix() string {
	return e.ID + strconv.Itoa(int(e.BatchesCreated.Add(1)))
}

// ConflictBatchWithPrefix generates a new conflict batch with scenario prefix created from scenario ID and batch count.
// BatchOutputs are outputs of the batch that can be reused in deep spamming by collecting them in Reuse wallet.
func (e *EvilScenario) ConflictBatchWithPrefix() (prefixedBatch EvilBatch, allAliases ScenarioAlias, batchOutputs map[string]types.Empty) {
	allAliases = NewScenarioAlias()
	prefix := e.nextBatchPrefix()
	for _, conflictMap := range e.ConflictBatch {
		scenarioAlias := make([]ScenarioAlias, 0)
		for _, aliases := range conflictMap {
			sa := NewScenarioAlias()
			for _, in := range aliases.Inputs {
				sa.Inputs = append(sa.Inputs, prefix+in)
				allAliases.Inputs = append(allAliases.Inputs, prefix+in)
			}
			for _, out := range aliases.Outputs {
				sa.Outputs = append(sa.Outputs, prefix+out)
				allAliases.Outputs = append(allAliases.Outputs, prefix+out)
			}
			scenarioAlias = append(scenarioAlias, sa)
		}
		prefixedBatch = append(prefixedBatch, scenarioAlias)
	}
	batchOutputs = e.readCustomConflictsPattern(prefixedBatch)
	return
}
