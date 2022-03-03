package evilwallet

import (
	"sync"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
)

type WalletType int8

const (
	fresh WalletType = iota
	noConflicts
	conflicts
)

// region Wallets ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Wallets struct {
	wallets map[WalletType][]*Wallet
	mu      sync.Mutex

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
	w.wallets[wallet.walletType] = append(w.wallets[wallet.walletType], wallet)
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
	spent           bool
	*sync.RWMutex
}

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

func (w *Wallet) Address() address.Address {
	w.lastAddrIdxUsed.Add(1)
	index := uint64(w.lastAddrIdxUsed.Load())
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.addrIndexMap[addr.Base58()] = index

	return addr
}

func (w *Wallet) AddUnspentOutput(addr ledgerstate.Address, outputID ledgerstate.OutputID, balance uint64) {
	w.Lock()
	defer w.Unlock()

	w.unspentOutputs[addr.Base58()] = &Output{
		OutputID: outputID,
		Address:  addr,
		Balance:  balance,
		Status:   pending,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
func (w *Wallet) sign(index uint64, txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	kp := w.seed.KeyPair(index)
	return ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
}
