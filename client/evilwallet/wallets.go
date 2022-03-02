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
	idxSpent := atomic.NewInt64(-1)
	addrUsed := atomic.NewInt64(-1)
	wallet := &Wallet{
		ID:                int(w.lastWalletID.Load()),
		walletType:        wType,
		seed:              seed.NewSeed(),
		unspentOutputs:    make(map[string]*Output),
		indexAddrMap:      make(map[uint64]string),
		inputTransactions: make(map[string]types.Empty),
		lastAddrSpent:     *idxSpent,
		lastAddrIdxUsed:   *addrUsed,
		RWMutex:           &sync.RWMutex{},
	}

	w.lastWalletID.Add(1)
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

func (w *Wallet) AddUnspentOutput(addr ledgerstate.Address, idx uint64, outputID ledgerstate.OutputID, balance uint64) {
	w.Lock()
	defer w.Unlock()

	w.unspentOutputs[addr.Base58()] = &Output{
		OutputID: outputID,
		Address:  addr,
		Balance:  balance,
		Status:   pending,
	}
}

// addWallet stores newly created wallet.
func (w *Wallets) addWallet(wallet *Wallet) {
	w.wallets[wallet.walletType] = append(w.wallets[wallet.walletType], wallet)
}


type Wallet struct {
	ID         int
	walletType WalletType

	unspentOutputs    map[string]*Output // maps addr to its unspentOutput
	indexAddrMap      map[uint64]string
	inputTransactions map[string]types.Empty
	seed              *seed.Seed

	lastAddrIdxUsed atomic.Int64 // used during filling in wallet with new outputs
	lastAddrSpent   atomic.Int64 // used during spamming with outputs one by one
	spent           bool
	*sync.RWMutex
}

func (w *Wallet) Address() address.Address {
	index := uint64(w.lastAddrIdxUsed.Add(1))
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()

	return addr
}
