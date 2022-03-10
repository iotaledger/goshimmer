package evilwallet

import (
	"sync"

	"github.com/cockroachdb/errors"

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
		lastWalletID: *atomic.NewInt64(-1),
	}
}

func (w *Wallets) NewWallet(wType WalletType) *Wallet {
	wallet := NewWallet(wType)
	wallet.ID = int(w.lastWalletID.Add(1))

	w.addWallet(wallet)

	return wallet
}

func (w *Wallets) GetWallet(walletType WalletType, walletID int) *Wallet {
	if len(w.wallets[walletType]) > walletID {
		return w.wallets[walletType][walletID]
	}
	return nil
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
	index := uint64(w.lastAddrIdxUsed.Add(1))
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.addrIndexMap[addr.Base58()] = index

	return addr
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
func (w *Wallet) AddUnspentOutput(addr ledgerstate.Address, addrIdx uint64, outputID ledgerstate.OutputID, balance *ledgerstate.ColoredBalances) *Output {
	w.Lock()
	defer w.Unlock()

	out := &Output{
		OutputID: outputID,
		Address:  addr,
		Index:    addrIdx,
		WalletID: w.ID,
		Balance:  balance,
		Status:   pending,
	}
	w.unspentOutputs[addr.Base58()] = out
	return out
}

func (w *Wallet) UnspentOutputBalance(addr string) *ledgerstate.ColoredBalances {
	if out, ok := w.unspentOutputs[addr]; ok {
		return out.Balance
	}
	return &ledgerstate.ColoredBalances{}
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
		return errors.Newf("could not find unspent output under provided address in the wallet, outIdx:%d, addr: %s", outputID, addr)
	}
	w.Lock()
	walletOutput.OutputID = outputID
	w.Unlock()
	return nil
}

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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
