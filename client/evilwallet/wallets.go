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
	Other WalletType = iota
	// Fresh is used for automatic Faucet Requests, outputs are returned one by one
	Fresh
	// Reuse stores resulting outputs of double spends or transactions issued by the evilWallet,
	// outputs from this wallet are reused in spamming scenario with flag reuse set to true and no RestrictedReuse wallet provided.
	// Reusing spammed outputs allows for creation of deeper UTXO DAG structure.
	Reuse
	// RestrictedReuse it is a reuse wallet, that will be available only to spamming scenarios that
	// will provide RestrictedWallet for the reuse spamming.
	RestrictedReuse
)

// region Wallets ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Wallets is a container of wallets.
type Wallets struct {
	wallets map[walletID]*Wallet
	// we store here non-empty wallets ids of wallets with Fresh faucet outputs.
	faucetWallets []walletID
	// reuse wallets are stored without an order, so they are picked up randomly.
	// Boolean flag indicates if wallet is ready - no new addresses will be generated, so empty wallets can be deleted.
	reuseWallets map[walletID]bool
	mu           sync.RWMutex

	lastWalletID atomic.Int64
}

// NewWallets creates and returns a new Wallets container.
func NewWallets() *Wallets {
	return &Wallets{
		wallets:       make(map[walletID]*Wallet),
		faucetWallets: make([]walletID, 0),
		reuseWallets:  make(map[walletID]bool),
		lastWalletID:  *atomic.NewInt64(-1),
	}
}

// NewWallet adds a new wallet to Wallets and returns the created wallet.
func (w *Wallets) NewWallet(walletType WalletType) *Wallet {
	wallet := NewWallet(walletType)
	wallet.ID = walletID(w.lastWalletID.Add(1))

	w.addWallet(wallet)
	if walletType == Reuse {
		w.addReuseWallet(wallet)
	}
	return wallet
}

// GetWallet returns the wallet with the specified ID.
func (w *Wallets) GetWallet(walletID walletID) *Wallet {
	return w.wallets[walletID]
}

// GetNextWallet get next non-empty wallet based on provided type.
func (w *Wallets) GetNextWallet(walletType WalletType, minOutputsLeft int) (*Wallet, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch walletType {
	case Fresh:
		if !w.IsFaucetWalletAvailable() {
			return nil, errors.New("no faucet wallets available, need to request more funds")
		}

		wallet := w.wallets[w.faucetWallets[0]]
		if wallet.IsEmpty() {
			return nil, errors.New("wallet is empty, need to request more funds")
		}
		return wallet, nil
	case Reuse:
		for id, ready := range w.reuseWallets {
			wal := w.wallets[id]
			if wal.UnspentOutputsLeft() > minOutputsLeft {
				// if are solid

				return wal, nil
			}
			// no outputs will be ever added to this wallet, so it can be deleted
			if wal.IsEmpty() && ready {
				w.removeReuseWallet(id)
			}
		}
		return nil, errors.New("no reuse wallets available")
	}

	return nil, errors.New("wallet type not supported for ordered usage, use GetWallet by ID instead")
}

func (w *Wallets) UnspentOutputsLeft(walletType WalletType) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	outputsLeft := 0

	switch walletType {
	case Fresh:
		for _, wID := range w.faucetWallets {
			outputsLeft += w.wallets[wID].UnspentOutputsLeft()
		}
	case Reuse:
		for wID := range w.reuseWallets {
			outputsLeft += w.wallets[wID].UnspentOutputsLeft()
		}
	}
	return outputsLeft
}

// addWallet stores newly created wallet.
func (w *Wallets) addWallet(wallet *Wallet) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.wallets[wallet.ID] = wallet

}

// addReuseWallet stores newly created wallet.
func (w *Wallets) addReuseWallet(wallet *Wallet) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.reuseWallets[wallet.ID] = false
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

// freshWallet returns the first non-empty wallet from the faucetWallets queue. If current wallet is empty,
// it is removed and the next enqueued one is returned.
func (w *Wallets) freshWallet() (*Wallet, error) {
	wallet, err := w.GetNextWallet(Fresh, 1)
	if err != nil {
		w.removeFreshWallet()
		wallet, err = w.GetNextWallet(Fresh, 1)
		if err != nil {
			return nil, err
		}
	}
	return wallet, nil
}

// reuseWallet returns the first non-empty wallet from the reuseWallets queue. If current wallet is empty,
// it is removed and the next enqueued one is returned.
func (w *Wallets) reuseWallet(outputsNeeded int) *Wallet {
	wallet, err := w.GetNextWallet(Reuse, outputsNeeded)
	if err != nil {
		return nil
	}
	return wallet
}

// removeWallet removes wallet, for Fresh wallet it will be the first wallet in a queue.
func (w *Wallets) removeFreshWallet() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.IsFaucetWalletAvailable() {
		removedID := w.faucetWallets[0]
		w.faucetWallets = w.faucetWallets[1:]
		delete(w.wallets, removedID)
	}
	return
}

// removeWallet removes wallet, for Fresh wallet it will be the first wallet in a queue.
func (w *Wallets) removeReuseWallet(walletID walletID) {
	if _, ok := w.reuseWallets[walletID]; ok {
		delete(w.reuseWallets, walletID)
		delete(w.wallets, walletID)
	}
	return
}

// SetWalletReady makes wallet ready to use, Fresh wallet is added to freshWallets queue.
func (w *Wallets) SetWalletReady(wallet *Wallet) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if wallet.IsEmpty() {
		return
	}
	wType := wallet.walletType
	switch wType {
	case Fresh:
		w.faucetWallets = append(w.faucetWallets, wallet.ID)
	case Reuse:
		w.reuseWallets[wallet.ID] = true
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
	reuseAddressPool  map[string]types.Empty
	seed              *seed.Seed

	lastAddrIdxUsed atomic.Int64 // used during filling in wallet with new outputs
	lastAddrSpent   atomic.Int64 // used during spamming with outputs one by one

	*sync.RWMutex
}

// NewWallet creates a wallet of a given type.
func NewWallet(wType ...WalletType) *Wallet {
	walletType := Other
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

	if walletType == Reuse {
		wallet.reuseAddressPool = make(map[string]types.Empty)
	}
	return wallet
}

// Type returns the wallet type.
func (w *Wallet) Type() WalletType {
	return w.walletType
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

// AddrIndexMap returns the index for the address specified.
func (w *Wallet) AddrIndexMap(outIndex string) uint64 {
	w.RLock()
	defer w.RUnlock()

	return w.addrIndexMap[outIndex]
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
func (w *Wallet) IsEmpty() (empty bool) {
	switch w.walletType {
	case Reuse:
		empty = len(w.reuseAddressPool) == 0
	default:
		empty = w.lastAddrSpent.Load() == w.lastAddrIdxUsed.Load() || w.UnspentOutputsLength() == 0
	}
	return
}

// UnspentOutputsLeft returns how many unused outputs are available in wallet.
func (w *Wallet) UnspentOutputsLeft() (left int) {
	switch w.walletType {
	case Reuse:
		left = len(w.reuseAddressPool)
	default:
		left = int(w.lastAddrIdxUsed.Load() - w.lastAddrSpent.Load())
	}
	return
}

// AddReuseAddress adds address to the reuse ready outputs' addresses pool for a Reuse wallet.
func (w *Wallet) AddReuseAddress(addr string) {
	w.Lock()
	defer w.Unlock()

	if w.walletType == Reuse {
		w.reuseAddressPool[addr] = types.Void
	}
}

// GetReuseAddress get random address from reuse addresses reuseOutputsAddresses pool. Address is removed from the pool after selecting.
func (w *Wallet) GetReuseAddress() string {
	w.Lock()
	defer w.Unlock()

	if w.walletType == Reuse {
		if len(w.reuseAddressPool) > 0 {
			for addr := range w.reuseAddressPool {
				delete(w.reuseAddressPool, addr)
				return addr
			}
		}
	}
	return ""
}

// GetUnspentOutput returns an unspent output on the oldest address ordered by index.
func (w *Wallet) GetUnspentOutput() *Output {
	switch w.walletType {
	case Reuse:
		addr := w.GetReuseAddress()
		return w.UnspentOutput(addr)
	default:
		if w.lastAddrSpent.Load() < w.lastAddrIdxUsed.Load() {
			idx := w.lastAddrSpent.Add(1)
			addr := w.IndexAddrMap(uint64(idx))
			out := w.UnspentOutput(addr)
			return out
		}
	}
	return nil
}

// Sign signs the tx essence.
func (w *Wallet) Sign(addr ledgerstate.Address, txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	w.RLock()
	defer w.RUnlock()
	index := w.AddrIndexMap(addr.Base58())
	kp := w.seed.KeyPair(index)
	return ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
}

// UpdateUnspentOutputID updates the unspent output on the address specified.
func (w *Wallet) UpdateUnspentOutputID(addr string, outputID ledgerstate.OutputID) error {
	w.RLock()
	walletOutput, ok := w.unspentOutputs[addr]
	w.RUnlock()
	if !ok {
		return errors.Newf("could not find unspent output under provided address in the wallet, outID:%s, addr: %s", outputID.Base58(), addr)
	}
	w.Lock()
	walletOutput.OutputID = outputID
	w.Unlock()
	return nil
}

// UnspentOutputsLength returns the number of unspent outputs on the wallet.
func (w *Wallet) UnspentOutputsLength() int {
	return len(w.unspentOutputs)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
