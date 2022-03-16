package evilwallet

import (
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type walletID int

// WalletType is the type of the wallet.
type WalletType int8
type WalletStatus int8

const (
	other WalletType = iota
	// fresh is used for automatic Faucet Requests, outputs are returned one by one
	fresh
	// reuse stores resulting outputs of double spends or transactions issued by the evilWallet
	reuse
)

// region Wallets ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Wallets is a container of wallets.
type Wallets struct {
	wallets map[walletID]*Wallet
	// we store here non-empty wallets ids of wallets with fresh faucet outputs.
	faucetWallets []walletID
	reuseWallets  []walletID
	mu            sync.RWMutex

	lastWalletID atomic.Int64
}

// NewWallets creates and returns a new Wallets container.
func NewWallets() *Wallets {
	return &Wallets{
		wallets:       make(map[walletID]*Wallet),
		faucetWallets: make([]walletID, 0),
		lastWalletID:  *atomic.NewInt64(-1),
	}
}

// NewWallet adds a new wallet to Wallets and returns the created wallet.
func (w *Wallets) NewWallet(walletType WalletType) *Wallet {
	wallet := NewWallet(walletType)
	wallet.ID = walletID(w.lastWalletID.Add(1))

	w.addWallet(wallet)

	return wallet
}

// GetWallet returns the wallet with the specified ID.
func (w *Wallets) GetWallet(walletID walletID) *Wallet {
	return w.wallets[walletID]
}

// GetNextWallet get next non-empty wallet based on provided type.
func (w *Wallets) GetNextWallet(walletType WalletType) (*Wallet, error) {
	if !w.IsFaucetWalletAvailable() {
		return nil, errors.New("no faucet wallets available, need to request more funds")
	}
	wallet := w.wallets[w.faucetWallets[0]]
	if wallet.IsEmpty() {
		return nil, errors.New("wallet is empty, need to request more funds")
	}
	return wallet, nil
}

// addWallet stores newly created wallet.
func (w *Wallets) addWallet(wallet *Wallet) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.wallets[wallet.ID] = wallet

}

// GetUnspentOutput gets first found unspent output for a given walletType
func (w *Wallets) GetUnspentOutput(wallet *Wallet) *Output {
	if wallet == nil {
		return nil
	}
	return wallet.GetUnspentOutput()
}

// IsFaucetWalletAvailable checks if there is any faucet wallet left.
func (w *Wallets) IsFaucetWalletAvailable() bool {
	return len(w.faucetWallets) > 0
}

// FreshWallet returns the first non-empty wallet from the faucetWallets queue. If current wallet is empty,
// it is removed and the next enqueued one is returned.
func (w *Wallets) FreshWallet() (*Wallet, error) {
	wallet, err := w.GetNextWallet(fresh)
	if err != nil {
		w.removeWallet(fresh)
		wallet, err = w.GetNextWallet(fresh)
		if err != nil {
			return nil, err
		}
	}
	return wallet, nil
}

// removeWallet removes wallet, for fresh wallet it will be the first wallet in a queue.
func (w *Wallets) removeWallet(wType WalletType) {
	switch wType {
	case fresh:
		if w.IsFaucetWalletAvailable() {
			w.faucetWallets = w.faucetWallets[1:]
		}
	}
	return
}

// SetWalletReady makes wallet ready to use, fresh wallet is added to freshWallets queue.
func (w *Wallets) SetWalletReady(wallet *Wallet) {
	wType := wallet.walletType
	switch wType {
	case fresh:
		w.faucetWallets = append(w.faucetWallets, wallet.ID)
	case reuse:
		w.reuseWallets = append(w.reuseWallets, wallet.ID)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region Wallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Wallet is the definition of a wallet.
type Wallet struct {
	ID                walletID
	walletType        WalletType
	unspentOutputs    map[string]*Output // maps addr to its unspentOutput
	indexAddrMap      map[uint64]string
	addrIndexMap      map[string]uint64
	inputTransactions map[string]types.Empty
	seed              *seed.Seed

	lastAddrIdxUsed atomic.Int64 // used during filling in wallet with new outputs
	lastAddrSpent   atomic.Int64 // used during spamming with outputs one by one

	*sync.RWMutex
}

// NewWallet creates a wallet of a given type.
func NewWallet(wType ...WalletType) *Wallet {
	walletType := other
	if len(wType) > 0 {
		walletType = wType[0]
	}
	idxSpent := atomic.NewInt64(-1)
	addrUsed := atomic.NewInt64(-1)
	wallet := &Wallet{
		walletType:        walletType,
		ID:                -1,
		seed:              seed.NewSeed(),
		unspentOutputs:    make(map[string]*Output),
		indexAddrMap:      make(map[uint64]string),
		addrIndexMap:      make(map[string]uint64),
		inputTransactions: make(map[string]types.Empty),
		lastAddrSpent:     *idxSpent,
		lastAddrIdxUsed:   *addrUsed,
		RWMutex:           &sync.RWMutex{},
	}

	return wallet
}

// Address returns a new and unused address of a given wallet.
func (w *Wallet) Address() address.Address {
	w.Lock()
	defer w.Unlock()

	index := uint64(w.lastAddrIdxUsed.Add(1))
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.addrIndexMap[addr.Base58()] = index

	return addr
}

// UnspentOutput returns the unspent output on the address.
func (w *Wallet) UnspentOutput(addr string) *Output {
	w.RLock()
	defer w.RUnlock()
	return w.unspentOutputs[addr]

}

// UnspentOutputs returns all unspent outputs on the wallet.
func (w *Wallet) UnspentOutputs() (outputs map[string]*Output) {
	w.RLock()
	defer w.RUnlock()
	outputs = make(map[string]*Output)
	for addr, out := range w.unspentOutputs {
		outputs[addr] = out
	}
	return outputs
}

// IndexAddrMap returns the address for the index specified.
func (w *Wallet) IndexAddrMap(outIndex uint64) string {
	w.RLock()
	defer w.RUnlock()

	return w.indexAddrMap[outIndex]
}

// AddUnspentOutput adds an unspentOutput of a given wallet.
func (w *Wallet) AddUnspentOutput(addr ledgerstate.Address, addrIdx uint64, outputID ledgerstate.OutputID, balance *ledgerstate.ColoredBalances) *Output {
	w.Lock()
	defer w.Unlock()

	out := &Output{
		OutputID: outputID,
		Address:  addr,
		Index:    addrIdx,
		Balance:  balance,
		Status:   pending,
	}
	w.unspentOutputs[addr.Base58()] = out
	return out
}

// UnspentOutputBalance returns the balance on the unspent output sitting on the address specified.
func (w *Wallet) UnspentOutputBalance(addr string) *ledgerstate.ColoredBalances {
	w.RLock()
	defer w.RUnlock()

	if out, ok := w.unspentOutputs[addr]; ok {
		return out.Balance
	}
	return &ledgerstate.ColoredBalances{}
}

// IsEmpty returns true if the wallet is empty.
func (w *Wallet) IsEmpty() bool {
	return w.lastAddrSpent.Load() == w.lastAddrIdxUsed.Load() || w.UnspentOutputsLength() == 0
}

// GetUnspentOutput returns an unspent output on the oldest address ordered by index.
func (w *Wallet) GetUnspentOutput() *Output {
	if w.lastAddrSpent.Load() < w.lastAddrIdxUsed.Load() {
		idx := w.lastAddrSpent.Add(1)
		addr := w.IndexAddrMap(uint64(idx))
		return w.UnspentOutput(addr)
	}
	return nil
}

// createOutputs creates n outputs by splitting the given balance equally between them.
func (w *Wallet) createOutputs(nOutputs int, inputBalance *ledgerstate.ColoredBalances) (outputs []ledgerstate.Output) {
	amount, _ := inputBalance.Get(ledgerstate.ColorIOTA)
	outputBalances := SplitBalanceEqually(nOutputs, amount)
	for i := 0; i < nOutputs; i++ {
		addr := w.Address()
		output := ledgerstate.NewSigLockedSingleOutput(outputBalances[i], addr.Address())
		outputs = append(outputs, output)
		// correct ID will be updated after txn confirmation
		w.AddUnspentOutput(addr.Address(), addr.Index, output.ID(), ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: outputBalances[i],
		}))
	}
	return
}

// sign signs the tx essence.
func (w *Wallet) sign(addr ledgerstate.Address, txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	index := w.addrIndexMap[addr.Base58()]
	kp := w.seed.KeyPair(index)
	return ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
}

// UpdateUnspentOutputID updates the unspent output on the address specified.
func (w *Wallet) UpdateUnspentOutputID(addr string, outputID ledgerstate.OutputID) error {
	w.RLock()
	walletOutput, ok := w.unspentOutputs[addr]
	w.RUnlock()
	if !ok {
		return errors.Newf("could not find unspent output under provided address in the wallet, outIdx:%d, addr: %s", outputID, addr)
	}
	w.Lock()
	walletOutput.OutputID = outputID
	w.Unlock()
	return nil
}

// UpdateUnspentOutputStatus updates the status of the unspent output on the address specified.
func (w *Wallet) UpdateUnspentOutputStatus(addr string, status OutputStatus) error {
	w.RLock()
	walletOutput, ok := w.unspentOutputs[addr]
	w.RUnlock()
	if !ok {
		return errors.Newf("could not find unspent output under provided address in the wallet, outIdx:%d, addr: %s", addr)
	}
	w.Lock()
	walletOutput.Status = status
	w.Unlock()
	return nil
}

// UnspentOutputsLength returns the number of unspent outputs on the wallet.
func (w *Wallet) UnspentOutputsLength() int {
	return len(w.unspentOutputs)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
