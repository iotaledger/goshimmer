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

// Output contains details of an output ID.
type Output struct {
	//*wallet.Output
	OutputID ledgerstate.OutputID
	Address  ledgerstate.Address
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

func (o *OutputManager) Track(outputIDs []ledgerstate.OutputID) (allConfirmed bool) {
	wg := sync.WaitGroup{}

	for _, ID := range outputIDs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok := o.AwaitOutputToBeConfirmed(ID, time.Duration(3*time.Second))
			if ok {
				o.status[ID].Status = confirmed
				return
			}
		}()
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

func (o *OutputManager) AddOutputsByAddress(address string) (outputIDs []ledgerstate.OutputID) {
	client := o.connector.GetClient()
	// add output to map
	res, err := client.GetAddressUnspentOutputs(address)
	if err != nil {
		return
	}

	outputIDs = o.AddOutputsByJson(res.Outputs)
	return outputIDs
}

func (o *OutputManager) AddOutputsByTxID(txID ledgerstate.TransactionID) (outputIDs []ledgerstate.OutputID) {
	client := o.connector.GetClient()
	// add output to map
	tx, err := client.GetTransaction(txID.Base58())
	if err != nil {
		return
	}

	outputIDs = o.AddOutputsByJson(tx.Outputs)
	return outputIDs
}

func (o *OutputManager) AddOutputsByJson(outputs []*jsonmodels.Output) (outputIDs []ledgerstate.OutputID) {
	for _, jsonOutput := range outputs {
		outputBytes, _ := jsonOutput.Output.MarshalJSON()
		output, _, err := ledgerstate.OutputFromBytes(outputBytes)
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
		addr := output.Address.Address()
		go func(addr ledgerstate.Address) {
			defer wg.Done()
			_, ok := o.AwaitUnspentOutputToBeConfirmed(addr, waitForConfirmation)
			if !ok {
				return
			}
		}(addr)
	}
	wg.Wait()
}

// AwaitUnspentOutputToBeConfirmed awaits for output from a provided address is confirmed. Timeout is waitFor.
// Useful when we have only an address and no transactionID, e.g. faucet funds request.
func (o *OutputManager) AwaitUnspentOutputToBeConfirmed(addr ledgerstate.Address, waitFor time.Duration) (outID string, ok bool) {
	s := time.Now()
	var confirmed bool
	for ; time.Since(s) < waitFor; time.Sleep(1 * time.Second) {
		jsonOutput := o.connector.GetUnspentOutputForAddress(addr)
		outID = jsonOutput.Output.OutputID.Base58
		confirmed = jsonOutput.GradeOfFinality == GoFConfirmed
		if outID != "" && confirmed {
			break
		}
	}
	if outID == "" {
		return
	}

	if !confirmed {
		return
	}
	ok = true
	return
}

// AwaitUnspentOutputToBeConfirmed awaits for output from a provided address is confirmed. Timeout is waitFor.
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

// AwaitTransactionsToBeConfirmed awaits for transaction confirmation and updates wallet with outputIDs.
func (o *OutputManager) AwaitTransactionsToBeConfirmed(wallet *Wallet, txIDs []string, maxGoroutines int) {
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
			// fill in wallet with outputs from confirmed transaction
			// todo finish when we will have wallet structure ready: o.getOutputsFromTransaction(wallet, txID)
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
