package evilwallet

import (
	"github.com/cockroachdb/errors"
	"sync"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
)

type WalletType int8

const (
	// fresh is used for automatic Faucet Requests, outputs are returned one by one
	fresh WalletType = iota
	// custom is used for manual handling of unspent outputs
	custom
	conflicts
)

// region Wallets ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Wallets struct {
	wallets map[WalletType][]*Wallet
	mu      sync.RWMutex

	lastWalletID atomic.Int64
}

func NewWallets() *Wallets {
	return &Wallets{
		wallets:      make(map[WalletType][]*Wallet),
		lastWalletID: *atomic.NewInt64(0),
	}
}

func (w *Wallets) NewWallet(wType WalletType) *Wallet {
	wallet := NewWallet(wType)
	wallet.ID = int(w.lastWalletID.Add(1))

	w.addWallet(wallet)

	return wallet
}

func (w *Wallets) GetWallet(wType WalletType) (wallet *Wallet) {
	return w.wallets[wType][0]
}

func (w *Wallets) GetWallets(wType WalletType, num int) (wallets []*Wallet) {
	allWallets := w.wallets[wType]
	if num > len(allWallets) {
		return allWallets
	}

	for i := 0; i < num; i++ {
		wallets = append(wallets, allWallets[i])
	}
	return wallets
}

// addWallet stores newly created wallet.
func (w *Wallets) addWallet(wallet *Wallet) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.wallets[wallet.walletType] = append(w.wallets[wallet.walletType], wallet)
}

// GetNonEmptyWallet returns first non-empty wallet.
func (w *Wallets) GetNonEmptyWallet(walletType WalletType) *Wallet {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, wallet := range w.wallets[walletType] {
		if !wallet.IsEmpty() {
			return wallet
		}
	}
	return nil
}

// GetUnspentOutput gets first found unspent output for a given walletType
func (w *Wallets) GetUnspentOutput(walletType WalletType) *Output {
	wallet := w.GetNonEmptyWallet(walletType)
	return wallet.GetUnspentOutput()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region Wallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Wallet struct {
	ID         int
	walletType WalletType

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
func NewWallet(wType WalletType) *Wallet {
	idxSpent := atomic.NewInt64(-1)
	addrUsed := atomic.NewInt64(-1)
	wallet := &Wallet{
		ID:                -1,
		walletType:        wType,
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
	w.lastAddrIdxUsed.Add(1)
	index := uint64(w.lastAddrIdxUsed.Load())
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.addrIndexMap[addr.Base58()] = index

	return addr
}

// AddressIndex returns a new and unused address of a given wallet along with address index.
func (w *Wallet) AddressIndex() (addr address.Address, index uint64) {
	w.lastAddrIdxUsed.Add(1)
	index = uint64(w.lastAddrIdxUsed.Load())
	addr = w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.addrIndexMap[addr.Base58()] = index

	return
}

func (w *Wallet) UnspentOutput(addr string) *Output {
	w.RLock()
	defer w.RUnlock()
	return w.unspentOutputs[addr]

}

func (w *Wallet) IndexAddrMap(outIndex uint64) string {
	w.RLock()
	defer w.RUnlock()

	return w.indexAddrMap[outIndex]
}

// AddUnspentOutput adds an unspentOutput of a given wallet.
func (w *Wallet) AddUnspentOutput(addr ledgerstate.Address, addrIdx uint64, outputID ledgerstate.OutputID, balance uint64) {
	w.Lock()
	defer w.Unlock()

	w.unspentOutputs[addr.Base58()] = &Output{
		OutputID: outputID,
		Address:  addr,
		Index:    addrIdx,
		WalletID: w.ID,
		Balance:  balance,
		Status:   pending,
	}
}

func (w *Wallet) UnspentOutputBalance(addr string) uint64 {
	if out, ok := w.unspentOutputs[addr]; ok {
		return out.Balance
	}
	return 0
}

func (w *Wallet) IsEmpty() bool {
	return w.lastAddrSpent.Load() == w.lastAddrIdxUsed.Load()
}

func (w *Wallet) GetUnspentOutput() *Output {
	if w.lastAddrSpent.Load() < w.lastAddrIdxUsed.Load() {
		idx := w.lastAddrSpent.Add(1)
		addr := w.IndexAddrMap(uint64(idx))
		return w.UnspentOutput(addr)
	}
	return nil
}

func (w *Wallet) createOutputs(nOutputs int, inputBalance uint64) (outputs []ledgerstate.Output) {
	outputBalances := SplitBalanceEqually(nOutputs, inputBalance)
	for i := 0; i < nOutputs; i++ {
		addr, idx := w.AddressIndex()
		output := ledgerstate.NewSigLockedSingleOutput(outputBalances[i], addr.Address())
		outputs = append(outputs, output)
		// correct ID will be updated after txn confirmation
		w.AddUnspentOutput(addr.Address(), idx, output.ID(), outputBalances[i])
	}
	return
}

func (w *Wallet) sign(addr ledgerstate.Address, txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	index := w.addrIndexMap[addr.Base58()]
	kp := w.seed.KeyPair(index)
	return ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
}

func (w *Wallet) UpdateUnspentOutputID(addr string, outputID ledgerstate.OutputID) error {
	w.RLock()
	walletOutput, ok := w.unspentOutputs[addr]
	w.RUnlock()
	if !ok {
		return errors.Newf("could not find unspent output ander provided address in the wallet, outIdx:%d, addr: %s", outputID, addr)
	}
	w.Lock()
	walletOutput.OutputID = outputID
	w.Unlock()
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
