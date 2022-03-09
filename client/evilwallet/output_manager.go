package evilwallet

import (
	"fmt"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
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

	wallets           *Wallets
	outputIDWalletMap map[ledgerstate.OutputID]*Wallet
	outputIDAddrMap   map[ledgerstate.OutputID]string
}

// NewOutputManager creates an OutputManager instance.
func NewOutputManager(connector Clients, wallets *Wallets) *OutputManager {
	return &OutputManager{
		connector:         connector,
		wallets:           wallets,
		outputIDWalletMap: make(map[ledgerstate.OutputID]*Wallet),
		outputIDAddrMap:   make(map[ledgerstate.OutputID]string),
	}
}

// Track the confirmed statuses of the given outputIDs, it returns true if all of them are confirmed.
func (o *OutputManager) Track(outputIDs []ledgerstate.OutputID) (allConfirmed bool) {
	wg := sync.WaitGroup{}
	allConfirmed = true
	for _, ID := range outputIDs {
		wg.Add(1)
		go func(id ledgerstate.OutputID, allConfirmed bool) {
			defer wg.Done()
			ok := o.AwaitOutputToBeConfirmed(id, awaitOutputToBeConfirmed)
			if !ok {
				allConfirmed = false
				return
			}
			_ = o.UpdateOutputStatus(id, confirmed)
		}(ID, allConfirmed)
	}
	return
}

// CreateEmptyOutput creates output without outputID, stores it in wallet w and return output instance.
// OutputManager maps are not updated, as outputID is not known yet.
func (o *OutputManager) CreateEmptyOutput(w *Wallet, balance uint64) *Output {
	addr := w.Address()
	out := w.AddUnspentOutput(addr.Address(), addr.Index, ledgerstate.OutputID{}, balance)
	return out
}

// CreateOutputFromAddress creates output, retrieves outputID, and adds it to the wallet.
// Provided address should be generated from provided wallet. Considers only first output found on address.
func (o *OutputManager) CreateOutputFromAddress(w *Wallet, addr address.Address, balance uint64) *Output {
	outputIDs := o.GetOutputsByAddress(addr.Base58())
	outputID := outputIDs[0]
	out := w.AddUnspentOutput(addr.Address(), addr.Index, outputID, balance)
	o.outputIDWalletMap[outputID] = w
	o.outputIDAddrMap[outputID] = addr.Base58()
	return out
}

func (o *OutputManager) CreateInput(w *Wallet) {

}

func (o *OutputManager) UpdateOutputID(w *Wallet, addr string, outID ledgerstate.OutputID) error {
	err := w.UpdateUnspentOutputID(addr, outID)
	o.outputIDWalletMap[outID] = w
	o.outputIDAddrMap[outID] = addr
	return err
}

func (o *OutputManager) UpdateOutputStatus(outID ledgerstate.OutputID, status OutputStatus) error {
	addr := o.outputIDAddrMap[outID]
	w := o.outputIDWalletMap[outID]
	err := w.UpdateUnspentOutputStatus(addr, status)
	o.outputIDAddrMap[outID] = addr
	return err
}

func (o *OutputManager) UpdateOutputsFromTxs(wallet *Wallet, txIDs []string) error {
	for _, txID := range txIDs {
		outputs, err := o.connector.GetTransactionOutputs(txID)
		if err != nil {
			return err
		}
		for _, out := range outputs {
			err = o.UpdateOutputID(wallet, out.Address().Base58(), out.ID())
			if err != nil {
				return err
			}
			err = o.UpdateOutputStatus(out.ID(), confirmed)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetOutput returns the Output of the given outputID.
func (o *OutputManager) GetOutput(outputID ledgerstate.OutputID) (output *Output) {
	w := o.outputIDWalletMap[outputID]
	addr := o.outputIDAddrMap[outputID]

	return w.UnspentOutput(addr)
}

// GetOutputsByAddress finds the unspent outputs of a given address and updates the provided output status map.
func (o *OutputManager) GetOutputsByAddress(address string) (outputIDs []ledgerstate.OutputID) {
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

	outputIDs = o.getOutputIDsByJSON(outputs)

	return outputIDs
}

// GetOutputsByTxID adds the outputs of a given transaction to the output status map.
func (o *OutputManager) GetOutputsByTxID(txID string) (outputIDs []ledgerstate.OutputID) {
	resp, err := o.connector.GetTransaction(txID)
	if err != nil {
		return
	}

	outputIDs = o.getOutputIDsByJSON(resp.Outputs)

	return outputIDs
}

func (o *OutputManager) getOutputIDsByJSON(outputs []*jsonmodels.Output) (outputIDs []ledgerstate.OutputID) {
	for _, jsonOutput := range outputs {
		output, err := jsonOutput.ToLedgerstateOutput()
		if err != nil {
			continue
		}
		outputIDs = append(outputIDs, output.ID())
	}
	return outputIDs
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
			outputIDs := o.GetOutputsByAddress(addr.Base58())
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
