package utxoutil

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"golang.org/x/xerrors"
	"time"
)

// Builder builder implements a structure and interface to build transactions
// Initial input is a list of outputs which can be used as consumable inputs
// Balances are tracked by "consuming" tokens from inputs and then "spending" them to outputs
// "consuming" tokens from inputs means:
// - tracking "remaining" part of tokens in each consumable output
// - placing consumed token into the "consumedUnspent"
// - By SpendConsumedUnspent all tokens are cleared from "consumedUnspent" and can be used as a token payload to
//   the output
type Builder struct {
	version           ledgerstate.TransactionEssenceVersion
	accessPledgeID    identity.ID
	consensusPledgeID identity.ID
	timestamp         time.Time
	// tracks remaining tokens for each consumable output
	consumables []*ConsumableOutput
	outputs     []ledgerstate.Output
	// buffer of consumed but unspent yet tokens
	consumedUnspent map[ledgerstate.Color]uint64
}

func NewBuilder(inputs ...ledgerstate.Output) *Builder {
	ret := &Builder{
		timestamp:       time.Now(),
		consumables:     make([]*ConsumableOutput, len(inputs)),
		outputs:         make([]ledgerstate.Output, 0),
		consumedUnspent: make(map[ledgerstate.Color]uint64),
	}
	ret.consumables = NewConsumables(inputs...)
	return ret
}

func (b *Builder) WithVersion(v ledgerstate.TransactionEssenceVersion) *Builder {
	b.version = v
	return b
}

func (b *Builder) WithTime(t time.Time) *Builder {
	b.timestamp = t
	return b
}

func (b *Builder) WithAccessPledge(id identity.ID) *Builder {
	b.accessPledgeID = id
	return b
}

func (b *Builder) WithConsensusPledge(id identity.ID) *Builder {
	b.consensusPledgeID = id
	return b
}

func (b *Builder) AddOutputAndSpend(out ledgerstate.Output) error {
	b.SpendConsumedUnspent()
	return b.addOutput(out)
}

func (b *Builder) addOutput(out ledgerstate.Output) error {
	for _, o := range b.outputs {
		if out.Compare(o) == 0 {
			return xerrors.New("duplicate outputs not allowed")
		}
	}
	b.outputs = append(b.outputs, out)
	return nil
}

func (b *Builder) addToConsumedUnspent(bals map[ledgerstate.Color]uint64) {
	for col, bal := range bals {
		s, _ := b.consumedUnspent[col]
		b.consumedUnspent[col] = s + bal
	}
}

func (b *Builder) consumeAmounts(amounts map[ledgerstate.Color]uint64) bool {
	if !ConsumeMany(amounts, b.consumables...) {
		return false
	}
	b.addToConsumedUnspent(amounts)
	return true
}

func (b *Builder) ensureEnoughUnspendAmounts(amounts map[ledgerstate.Color]uint64) bool {
	missing := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if s, _ := b.consumedUnspent[col]; s < bal {
			missing[col] = bal - s
		}
	}
	if len(missing) > 0 {
		if !b.consumeAmounts(missing) {
			return false
		}
	}
	return true
}

func (b *Builder) mustSpendAmounts(amounts map[ledgerstate.Color]uint64) {
	for col, bal := range amounts {
		if s, _ := b.consumedUnspent[col]; s >= bal {
			if s == bal {
				delete(b.consumedUnspent, col)
			} else {
				b.consumedUnspent[col] = s - bal
			}
		} else {
			panic("not enough unspent amounts")
		}
	}
}

// SpendConsumedUnspent spends all wasConsumed unspent and return
func (b *Builder) SpendConsumedUnspent() map[ledgerstate.Color]uint64 {
	ret := b.consumedUnspent
	b.consumedUnspent = make(map[ledgerstate.Color]uint64)
	return ret
}

// prepareColoredBalancesOutput:
// - ensures enough tokens in unspentAmounts
// - spends them
// - handles minting of new colors (if any) and returns final map of tokens with valid minting
func (b *Builder) prepareColoredBalancesOutput(amounts map[ledgerstate.Color]uint64, mint ...uint64) (map[ledgerstate.Color]uint64, error) {
	if len(amounts) == 0 {
		return nil, xerrors.New("AddSigLockedColoredOutput: no tokens to transfer")
	}
	// check if it is enough wasConsumed unspent amounts
	if !b.ensureEnoughUnspendAmounts(amounts) {
		return nil, xerrors.New("AddSigLockedColoredOutput: not enough balance")
	}
	b.mustSpendAmounts(amounts)
	amountsCopy := make(map[ledgerstate.Color]uint64)
	for col, bal := range amounts {
		if bal == 0 {
			return nil, xerrors.New("AddSigLockedColoredOutput: zero tokens in input not allowed")
		}
		amountsCopy[col] = bal
	}
	iotas, _ := amountsCopy[ledgerstate.ColorIOTA]
	if len(mint) > 0 && mint[0] > iotas {
		return nil, xerrors.Errorf("can't mint more tokens (%d) than wasConsumed iotas (%d)", iotas, mint[0])
	}
	if len(mint) > 0 && mint[0] > 0 {
		amountsCopy[ledgerstate.ColorMint] = mint[0]
		if iotas > mint[0] {
			amountsCopy[ledgerstate.ColorIOTA] = iotas - mint[0]
		}
	}
	return amountsCopy, nil
}

func (b *Builder) AddSigLockedColoredOutput(targetAddress ledgerstate.Address, amounts map[ledgerstate.Color]uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(amounts, mint...)
	if err != nil {
		return err
	}
	bals := ledgerstate.NewColoredBalances(balances)
	output := ledgerstate.NewSigLockedColoredOutput(bals, targetAddress)
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

// AddSigLockedIOTAOutput adds output with iotas by consuming inputs
// supports minting (coloring) of part of wasConsumed iotas
func (b *Builder) AddSigLockedIOTAOutput(targetAddress ledgerstate.Address, amount uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: amount}, mint...)
	if err != nil {
		return err
	}
	var output ledgerstate.Output
	if _, ok := balances[ledgerstate.ColorMint]; ok {
		output = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(balances), targetAddress)
	} else {
		output = ledgerstate.NewSigLockedSingleOutput(amount, targetAddress)
	}
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

func (b *Builder) AddExtendedOutputSimple(targetAddress ledgerstate.Address, amounts map[ledgerstate.Color]uint64, mint ...uint64) error {
	balances, err := b.prepareColoredBalancesOutput(amounts, mint...)
	if err != nil {
		return err
	}
	output := ledgerstate.NewExtendedLockedOutput(balances, targetAddress)
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

func (b *Builder) AddReminderOutputIfNeeded(reminderAddr ledgerstate.Address, compress ...bool) error {
	compr := false
	if len(compress) > 0 {
		compr = compress[0]
	}
	b.ConsumeReminderBalances(compr)
	unspent := b.ConsumedUnspent()
	if len(unspent) == 0 {
		// no need for reminder output
		return nil
	}
	return b.AddExtendedOutputSimple(reminderAddr, unspent)
}

// AddNewChainMint creates new self governed chain.
// The identity of the chain is not known until the full transaction is produced
func (b *Builder) AddNewChainMint(balances map[ledgerstate.Color]uint64, stateAddress ledgerstate.Address, stateData []byte) error {
	output, err := ledgerstate.NewChainOutputMint(balances, stateAddress)
	if err != nil {
		return err
	}
	if err := output.SetStateData(stateData); err != nil {
		return err
	}
	if !b.ensureEnoughUnspendAmounts(balances) {
		return xerrors.New("not enough tokens")
	}
	b.SpendConsumedUnspent()
	if err := b.addOutput(output); err != nil {
		return err
	}
	return nil
}

func (b *Builder) ConsumedUnspent() map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	for col, bal := range b.consumedUnspent {
		if bal != 0 {
			ret[col] = bal
		}
	}
	return ret
}

// ConsumeChainInputToOutput consumes and returns clone of the input
func (b *Builder) ConsumeChainInputToOutput(addressAlias ledgerstate.Address) (*ledgerstate.ChainOutput, error) {
	out, idx, ok := FindChainConsumableInput(addressAlias, b.consumables...)
	if !ok {
		return nil, xerrors.Errorf("can't find chain input for %s", addressAlias)
	}
	b.MustConsumeUntouchedInputByIndex(idx)
	return out.NewChainOutputNext(), nil
}

func (b *Builder) MustConsumeUntouchedInputByIndex(index int) {
	if index >= len(b.consumables) || b.consumables[index].WasConsumed() {
		panic("MustConsumeUntouchedInputByIndex: invalid consumable")
	}
	b.addToConsumedUnspent(ConsumeRemaining(b.consumables[index]))
}

func (b *Builder) ConsumeReminderBalances(compress bool) []*ConsumableOutput {
	inputConsumables := b.consumables
	if !compress {
		inputConsumables = SelectConsumed(b.consumables...)
	}
	b.addToConsumedUnspent(ConsumeRemaining(inputConsumables...))
	return inputConsumables
}

func (b *Builder) BuildEssence(compress ...bool) (*ledgerstate.TransactionEssence, []ledgerstate.Output, error) {
	if len(b.consumedUnspent) > 0 {
		return nil, nil, xerrors.New("not all consumed balances were spent")
	}
	compr := false
	if len(compress) > 0 {
		compr = compress[0]
	}
	inputConsumables := b.consumables
	if !compr {
		inputConsumables = SelectConsumed(b.consumables...)
	}
	for _, consumable := range inputConsumables {
		if !consumable.NothingRemains() {
			return nil, nil, xerrors.New("not all inputs were completely were consumed")
		}
	}
	// NewOutputs sorts the outputs and changes indices -> impossible to know indexUnlocked of a particular output
	outputs := ledgerstate.NewOutputs(b.outputs...)
	inputs, consumedOutputs := MakeUTXOInputs(inputConsumables...)
	ret := ledgerstate.NewTransactionEssence(b.version, b.timestamp, b.accessPledgeID, b.consensusPledgeID, inputs, outputs)
	return ret, consumedOutputs, nil
}

func (b *Builder) BuildWithED25519(keyPairs ...*ed25519.KeyPair) (*ledgerstate.Transaction, error) {
	essence, consumedOutputs, err := b.BuildEssence()
	if err != nil {
		return nil, err
	}
	unlockBlocks, err := UnlockInputsWithED25519KeyPairs(consumedOutputs, essence, keyPairs)
	return ledgerstate.NewTransaction(essence, unlockBlocks), nil
}
