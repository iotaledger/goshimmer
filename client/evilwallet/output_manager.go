package evilwallet

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/ds/types"
)

var (
	awaitOutputsByAddress    = 150 * time.Second
	awaitOutputToBeConfirmed = 150 * time.Second
)

// Output contains details of an output ID.
type Output struct {
	// *wallet.Output
	OutputID utxo.OutputID
	Address  devnetvm.Address
	Index    uint64
	Balance  *devnetvm.ColoredBalances
}

// Outputs is a list of Output.
type Outputs []*Output

// OutputManager keeps track of the output statuses.
type OutputManager struct {
	connector Connector

	wallets           *Wallets
	outputIDWalletMap map[string]*Wallet
	outputIDAddrMap   map[string]string
	// stores solid outputs per node
	issuerSolidOutIDMap map[string]map[string]types.Empty

	sync.RWMutex
}

// NewOutputManager creates an OutputManager instance.
func NewOutputManager(connector Connector, wallets *Wallets) *OutputManager {
	return &OutputManager{
		connector:           connector,
		wallets:             wallets,
		outputIDWalletMap:   make(map[string]*Wallet),
		outputIDAddrMap:     make(map[string]string),
		issuerSolidOutIDMap: make(map[string]map[string]types.Empty),
	}
}

// setOutputIDWalletMap sets wallet for the provided outputID.
func (o *OutputManager) setOutputIDWalletMap(outputID string, wallet *Wallet) {
	o.Lock()
	defer o.Unlock()

	o.outputIDWalletMap[outputID] = wallet
}

// setOutputIDAddrMap sets address for the provided outputID.
func (o *OutputManager) setOutputIDAddrMap(outputID string, addr string) {
	o.Lock()
	defer o.Unlock()

	o.outputIDAddrMap[outputID] = addr
}

// OutputIDWalletMap returns wallet corresponding to the outputID stored in OutputManager.
func (o *OutputManager) OutputIDWalletMap(outputID string) *Wallet {
	o.RLock()
	defer o.RUnlock()

	return o.outputIDWalletMap[outputID]
}

// OutputIDAddrMap returns address corresponding to the outputID stored in OutputManager.
func (o *OutputManager) OutputIDAddrMap(outputID string) (addr string) {
	o.RLock()
	defer o.RUnlock()

	addr = o.outputIDAddrMap[outputID]
	return
}

// SetOutputIDSolidForIssuer sets solid flag for the provided outputID and issuer.
func (o *OutputManager) SetOutputIDSolidForIssuer(outputID string, issuer string) {
	o.Lock()
	defer o.Unlock()

	if _, ok := o.issuerSolidOutIDMap[issuer]; !ok {
		o.issuerSolidOutIDMap[issuer] = make(map[string]types.Empty)
	}
	o.issuerSolidOutIDMap[issuer][outputID] = types.Void
}

// IssuerSolidOutIDMap checks whether output was marked as solid for a given node.
func (o *OutputManager) IssuerSolidOutIDMap(issuer, outputID string) (isSolid bool) {
	o.RLock()
	defer o.RUnlock()

	if solidOutputs, ok := o.issuerSolidOutIDMap[issuer]; ok {
		if _, isSolid = solidOutputs[outputID]; isSolid {
			return
		}
	}
	return
}

// Track the confirmed statuses of the given outputIDs, it returns true if all of them are confirmed.
func (o *OutputManager) Track(outputIDs []utxo.OutputID) (allConfirmed bool) {
	var (
		wg                     sync.WaitGroup
		unconfirmedOutputFound atomic.Bool
	)

	for _, ID := range outputIDs {
		wg.Add(1)

		go func(id utxo.OutputID) {
			defer wg.Done()

			if !o.AwaitOutputToBeAccepted(id, awaitOutputToBeConfirmed) {
				unconfirmedOutputFound.Store(true)
			}
		}(ID)
	}
	wg.Wait()

	return !unconfirmedOutputFound.Load()
}

// CreateOutputFromAddress creates output, retrieves outputID, and adds it to the wallet.
// Provided address should be generated from provided wallet. Considers only first output found on address.
func (o *OutputManager) CreateOutputFromAddress(w *Wallet, addr address.Address, balance *devnetvm.ColoredBalances) *Output {
	outputIDs := o.RequestOutputsByAddress(addr.Base58())
	if len(outputIDs) == 0 {
		return nil
	}
	outputID := outputIDs[0]
	out := w.AddUnspentOutput(addr.Address(), addr.Index, outputID, balance)
	o.setOutputIDWalletMap(outputID.Base58(), w)
	o.setOutputIDAddrMap(outputID.Base58(), addr.Base58())
	return out
}

// AddOutput adds existing output from wallet w to the OutputManager.
func (o *OutputManager) AddOutput(w *Wallet, output devnetvm.Output) *Output {
	outputID := output.ID()
	idx := w.AddrIndexMap(output.Address().Base58())
	out := w.AddUnspentOutput(output.Address(), idx, outputID, output.Balances())
	o.setOutputIDWalletMap(outputID.Base58(), w)
	o.setOutputIDAddrMap(outputID.Base58(), output.Address().Base58())
	return out
}

// GetOutput returns the Output of the given outputID.
// Firstly checks if output can be retrieved by outputManager from wallet, if not does an API call.
func (o *OutputManager) GetOutput(outputID utxo.OutputID) (output *Output) {
	output = o.getOutputFromWallet(outputID)

	// get output info via web api
	if output == nil {
		clt := o.connector.GetClient()
		out := clt.GetOutput(outputID)
		if out == nil {
			return nil
		}
		output = &Output{
			OutputID: out.ID(),
			Address:  out.Address(),
			Balance:  out.Balances(),
		}
	}

	return output
}

func (o *OutputManager) getOutputFromWallet(outputID utxo.OutputID) (output *Output) {
	o.RLock()
	defer o.RUnlock()
	w, ok := o.outputIDWalletMap[outputID.Base58()]
	if ok {
		addr := o.outputIDAddrMap[outputID.Base58()]
		output = w.UnspentOutput(addr)
	}
	return
}

// RequestOutputsByAddress finds the unspent outputs of a given address and updates the provided output status map.
func (o *OutputManager) RequestOutputsByAddress(address string) (outputIDs []utxo.OutputID) {
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
func (o *OutputManager) RequestOutputsByTxID(txID string) (outputIDs []utxo.OutputID) {
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
		go func(addr devnetvm.Address) {
			defer wg.Done()

			_ = o.Track(o.RequestOutputsByAddress(addr.Base58()))
		}(addr)
	}
	wg.Wait()
}

// AwaitOutputToBeAccepted awaits for output from a provided outputID is accepted. Timeout is waitFor.
// Useful when we have only an address and no transactionID, e.g. faucet funds request.
func (o *OutputManager) AwaitOutputToBeAccepted(outputID utxo.OutputID, waitFor time.Duration) (accepted bool) {
	s := time.Now()
	clt := o.connector.GetClient()
	accepted = false
	for ; time.Since(s) < waitFor; time.Sleep(awaitConfirmationSleep) {
		confirmationState := clt.GetOutputConfirmationState(outputID)
		if confirmationState.IsAccepted() {
			accepted = true
			break
		}
	}

	return accepted
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
			err := o.AwaitTransactionToBeAccepted(txID, waitForConfirmation)
			if err != nil {
				return
			}
		}(txID)
	}
	wg.Wait()
}

// AwaitTransactionToBeAccepted awaits for acceptance of a single transaction.
func (o *OutputManager) AwaitTransactionToBeAccepted(txID string, waitFor time.Duration) error {
	s := time.Now()
	clt := o.connector.GetClient()
	var accepted bool
	for ; time.Since(s) < waitFor; time.Sleep(awaitConfirmationSleep) {
		if confirmationState := clt.GetTransactionConfirmationState(txID); confirmationState.IsAccepted() {
			accepted = true
			break
		}
	}
	if !accepted {
		return errors.Errorf("transaction %s not accepted in time", txID)
	}
	return nil
}

// AwaitOutputToBeSolid awaits for solidification of a single output by provided clt.
func (o *OutputManager) AwaitOutputToBeSolid(outID string, clt Client, waitFor time.Duration) error {
	s := time.Now()
	var solid bool

	for ; time.Since(s) < waitFor; time.Sleep(awaitSolidificationSleep) {
		solid = o.IssuerSolidOutIDMap(clt.URL(), outID)
		if solid {
			break
		}
		if isSolid, _ := clt.GetOutputSolidity(outID); isSolid {
			solid = isSolid
			break
		}
	}
	if !solid {
		return errors.Errorf("output %s not solidified in time", outID)
	}
	return nil
}

// AwaitOutputsToBeSolid awaits for all provided outputs are solid for a provided client.
func (o *OutputManager) AwaitOutputsToBeSolid(outputs []string, clt Client, maxGoroutines int) (allSolid bool) {
	wg := sync.WaitGroup{}
	semaphore := make(chan bool, maxGoroutines)
	allSolid = true

	for _, outID := range outputs {
		wg.Add(1)
		go func(outID string) {
			defer wg.Done()
			semaphore <- true
			defer func() {
				<-semaphore
			}()
			err := o.AwaitOutputToBeSolid(outID, clt, waitForSolidification)
			if err != nil {
				allSolid = false
				return
			}
		}(outID)
	}
	wg.Wait()
	return
}
