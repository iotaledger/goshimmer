package evilwallet

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// OutputStatus represents the confirmation status of an output.
type OutputStatus int8

const (
	pending OutputStatus = iota
	confirmed
	rejected
)

var (
	awaitOutputsByAddress    = 3 * time.Second
	awaitOutputToBeConfirmed = 3 * time.Second
)

// Output contains details of an output ID.
type Output struct {
	//*wallet.Output
	OutputID ledgerstate.OutputID
	Address  ledgerstate.Address
	Index    uint64
	WalletID int
	Balance  uint64
	Status   OutputStatus
}

// Outputs is a list of Output.
type Outputs []*Output

// OutputManager keeps track of the output statuses.
type OutputManager struct {
	connector Clients

	status map[ledgerstate.OutputID]*Output
}

// NewOutputManager creates an OutputManager instance.
func NewOutputManager(connector Clients) *OutputManager {
	return &OutputManager{
		connector: connector,
		status:    make(map[ledgerstate.OutputID]*Output),
	}
}

// Track the confirmed statuses of the given outputIDs, it returns true if all of them are confirmed.
func (o *OutputManager) Track(outputIDs []ledgerstate.OutputID) (allConfirmed bool) {
	wg := sync.WaitGroup{}

	for _, ID := range outputIDs {
		wg.Add(1)
		go func(id ledgerstate.OutputID) {
			defer wg.Done()
			ok := o.AwaitOutputToBeConfirmed(id, awaitOutputToBeConfirmed)
			if ok {
				o.status[id].Status = confirmed
			}
		}(ID)
	}
	wg.Wait()

	allConfirmed = true
	for _, ID := range outputIDs {
		if o.status[ID].Status != confirmed {
			allConfirmed = false
			return
		}
	}
	return
}

// GetOutput returns the Output of the given outputID.
func (o *OutputManager) GetOutput(outputID ledgerstate.OutputID) (output *Output) {
	return o.status[outputID]
}

// AddOutputsByAddress finds the unspent outputs of a given address and add them to the output status map.
func (o *OutputManager) AddOutputsByAddress(address string) (outputIDs []ledgerstate.OutputID) {
	client := o.connector.GetClient()

	s := time.Now()
	var outputs []*jsonmodels.Output
	for ; time.Since(s) < awaitOutputsByAddress; time.Sleep(1 * time.Second) {
		res, err := client.GetAddressUnspentOutputs(address)
		if err == nil && len(res.Outputs) > 0 {
			outputs = res.Outputs
			break
		}
	}

	outputIDs = o.addOutputsByJSON(outputs)

	return outputIDs
}

// AddOutputsByTxID adds the outputs of a given transaction to the output status map.
func (o *OutputManager) AddOutputsByTxID(txID string) (outputIDs []ledgerstate.OutputID) {
	resp, err := o.connector.GetTransaction(txID)
	if err != nil {
		return
	}

	outputIDs = o.addOutputsByJSON(resp.Outputs)

	return outputIDs
}

func (o *OutputManager) addOutputsByJSON(outputs []*jsonmodels.Output) (outputIDs []ledgerstate.OutputID) {
	for _, jsonOutput := range outputs {
		output, err := jsonOutput.ToLedgerstateOutput()
		if err != nil {
			continue
		}
		o.AddOutput(output)
		outputIDs = append(outputIDs, output.ID())
	}
	return outputIDs
}

func (o *OutputManager) AddOutput(output ledgerstate.Output) {
	balance, _ := output.Balances().Get(ledgerstate.ColorIOTA)
	o.status[output.ID()] = &Output{
		OutputID: output.ID(),
		Address:  output.Address(),
		Balance:  balance,
		Status:   pending,
	}
}

// AwaitWalletOutputsToBeConfirmed awaits for all outputs in the wallet are confirmed.
func (o *OutputManager) AwaitWalletOutputsToBeConfirmed(wallet *Wallet) {
	wg := sync.WaitGroup{}
	for _, output := range wallet.unspentOutputs {
		wg.Add(1)
		if output == nil {
			continue
		}
		addr := output.Address
		go func(addr ledgerstate.Address) {
			defer wg.Done()
			outputIDs := o.AddOutputsByAddress(addr.Base58())
			ok := o.Track(outputIDs)
			if !ok {
				return
			}
		}(addr)
	}
	wg.Wait()
}

// AwaitOutputToBeConfirmed awaits for output from a provided outputID is confirmed. Timeout is waitFor.
// Useful when we have only an address and no transactionID, e.g. faucet funds request.
func (o *OutputManager) AwaitOutputToBeConfirmed(outputID ledgerstate.OutputID, waitFor time.Duration) (confirmed bool) {
	s := time.Now()
	confirmed = false
	for ; time.Since(s) < waitFor; time.Sleep(1 * time.Second) {
		gof := o.connector.GetOutputGoF(outputID)
		if gof == GoFConfirmed {
			confirmed = true
			break
		}
	}

	return confirmed
}

// AwaitTransactionsConfirmation awaits for transaction confirmation and updates wallet with outputIDs.
func (o *OutputManager) AwaitTransactionsConfirmation(txIDs []string, maxGoroutines int) {
	wg := sync.WaitGroup{}
	semaphore := make(chan bool, maxGoroutines)

	for _, txID := range txIDs {
		wg.Add(1)
		go func(txID string) {
			defer wg.Done()
			semaphore <- true
			defer func() {
				<-semaphore
			}()
			err := o.AwaitTransactionToBeConfirmed(txID, waitForConfirmation)
			if err != nil {
				return
			}
		}(txID)
	}
	wg.Wait()
}

// AwaitTransactionToBeConfirmed awaits for confirmation of a single transaction.
func (o *OutputManager) AwaitTransactionToBeConfirmed(txID string, waitFor time.Duration) error {
	s := time.Now()
	var confirmed bool
	for ; time.Since(s) < waitFor; time.Sleep(2 * time.Second) {
		if gof := o.connector.GetTransactionGoF(txID); gof == GoFConfirmed {
			confirmed = true
			break
		}
	}
	if !confirmed {
		return fmt.Errorf("transaction %s not confirmed in time", txID)
	}
	return nil
}
