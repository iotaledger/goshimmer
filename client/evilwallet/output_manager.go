package evilwallet

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"

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
	Balance  *ledgerstate.ColoredBalances
	Status   OutputStatus
}

// Outputs is a list of Output.
type Outputs []*Output

// OutputManager keeps track of the output statuses.
type OutputManager struct {
	connector Connector

	wallets           *Wallets
	outputIDWalletMap map[string]*Wallet
	outputIDAddrMap   map[ledgerstate.OutputID]string

	sync.RWMutex
}

// NewOutputManager creates an OutputManager instance.
func NewOutputManager(connector Connector, wallets *Wallets) *OutputManager {
	return &OutputManager{
		connector:         connector,
		wallets:           wallets,
		outputIDWalletMap: make(map[string]*Wallet),
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
func (o *OutputManager) CreateEmptyOutput(w *Wallet, balance *ledgerstate.ColoredBalances) *Output {
	addr := w.Address()
	out := w.AddUnspentOutput(addr.Address(), addr.Index, ledgerstate.OutputID{}, balance)
	return out
}

// CreateOutputFromAddress creates output, retrieves outputID, and adds it to the wallet.
// Provided address should be generated from provided wallet. Considers only first output found on address.
func (o *OutputManager) CreateOutputFromAddress(w *Wallet, addr address.Address, balance *ledgerstate.ColoredBalances) *Output {
	outputIDs := o.RequestOutputsByAddress(addr.Base58())
	if len(outputIDs) == 0 {
		return nil
	}
	outputID := outputIDs[0]
	out := w.AddUnspentOutput(addr.Address(), addr.Index, outputID, balance)
	o.Lock()
	o.outputIDWalletMap[outputID.Base58()] = w
	o.outputIDAddrMap[outputID] = addr.Base58()
	o.Unlock()
	return out
}

// UpdateOutputID updates the output wallet  and address.
func (o *OutputManager) UpdateOutputID(w *Wallet, addr string, outID ledgerstate.OutputID) error {
	err := w.UpdateUnspentOutputID(addr, outID)
	o.Lock()
	o.outputIDWalletMap[outID.Base58()] = w
	o.outputIDAddrMap[outID] = addr
	o.Unlock()
	return err
}

// UpdateOutputStatus updates the status of the outputID specified.
func (o *OutputManager) UpdateOutputStatus(outID ledgerstate.OutputID, status OutputStatus) error {
	o.RLock()
	addr := o.outputIDAddrMap[outID]
	w := o.outputIDWalletMap[outID.Base58()]
	o.RUnlock()
	err := w.UpdateUnspentOutputStatus(addr, status)

	return err
}

// UpdateOutputsFromTxs update the output maps from the status of the transactions specified.
func (o *OutputManager) UpdateOutputsFromTxs(txIDs []string) error {
	for _, txID := range txIDs {
		clt := o.connector.GetClient()
		outputs, err := clt.GetTransactionOutputs(txID)
		if err != nil {
			return err
		}
		for _, out := range outputs {
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
	o.RLock()
	defer o.RUnlock()
	w, ok := o.outputIDWalletMap[outputID.Base58()]
	if !ok {
		return nil
	}
	addr := o.outputIDAddrMap[outputID]
	out := w.UnspentOutput(addr)
	return out
}

func (o *OutputManager) GetWalletByOutputID(outID string) *Wallet {
	o.RLock()
	defer o.RUnlock()

	return o.outputIDWalletMap[outID]
}

// RequestOutputsByAddress finds the unspent outputs of a given address and updates the provided output status map.
func (o *OutputManager) RequestOutputsByAddress(address string) (outputIDs []ledgerstate.OutputID) {
	s := time.Now()
	clt := o.connector.GetClient()
	for ; time.Since(s) < awaitOutputsByAddress; time.Sleep(1 * time.Second) {
		outputIDs, err := clt.GetAddressUnspentOutputs(address)
		if err == nil && len(outputIDs) > 0 {
			return outputIDs
		}
	}

	return
}

// RequestOutputsByTxID adds the outputs of a given transaction to the output status map.
func (o *OutputManager) RequestOutputsByTxID(txID string) (outputIDs []ledgerstate.OutputID) {
	clt := o.connector.GetClient()
	resp, err := clt.GetTransaction(txID)
	if err != nil {
		return
	}

	outputIDs = getOutputIDsByJSON(resp.Outputs)

	return outputIDs
}

// AwaitWalletOutputsToBeConfirmed awaits for all outputs in the wallet are confirmed.
func (o *OutputManager) AwaitWalletOutputsToBeConfirmed(wallet *Wallet) {
	wg := sync.WaitGroup{}
	for _, output := range wallet.UnspentOutputs() {
		wg.Add(1)
		if output == nil {
			continue
		}
		addr := output.Address
		go func(addr ledgerstate.Address) {
			defer wg.Done()
			outputIDs := o.RequestOutputsByAddress(addr.Base58())
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
	clt := o.connector.GetClient()
	confirmed = false
	for ; time.Since(s) < waitFor; time.Sleep(1 * time.Second) {
		gof := clt.GetOutputGoF(outputID)
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
	clt := o.connector.GetClient()
	var confirmed bool
	for ; time.Since(s) < waitFor; time.Sleep(2 * time.Second) {
		if gof := clt.GetTransactionGoF(txID); gof == GoFConfirmed {
			confirmed = true
			break
		}
	}
	if !confirmed {
		return fmt.Errorf("transaction %s not confirmed in time", txID)
	}
	return nil
}
