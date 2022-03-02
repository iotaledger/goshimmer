package evilwallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
	"sync"
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

	idCounter    atomic.Int64
	lastWalletID atomic.Int64
}

func NewWallets() *Wallets {
	return &Wallets{
		wallets:      make(map[WalletType][]*Wallet),
		idCounter:    *atomic.NewInt64(0),
		lastWalletID: *atomic.NewInt64(0),
	}
}

type Wallet struct {
	ID         int64
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

func NewWallet() *Wallet {
	idxSpent := atomic.NewInt64(-1)
	addrUsed := atomic.NewInt64(-1)
	return &Wallet{
		ID:                -1,
		seed:              seed.NewSeed(),
		unspentOutputs:    make(map[string]*Output),
		indexAddrMap:      make(map[uint64]string),
		inputTransactions: make(map[string]types.Empty),
		lastAddrSpent:     *idxSpent,
		lastAddrIdxUsed:   *addrUsed,
		RWMutex:           &sync.RWMutex{},
	}
}

func (w *Wallet) SetID(id int64) {
	w.ID = id
}

func (w *Wallet) GenerateNextAddress() (ledgerstate.Address, uint64) {
	newIdx := uint64(w.lastAddrIdxUsed.Add(1))
	return w.seed.Address(newIdx).Address(), newIdx
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

	w.indexAddrMap[idx] = addr.Base58()
}

// AddWalletAndAssignID adds a new wallet to wallets and sets its ID with the next global numbering.
func (e *Wallets) AddWalletAndAssignID(wallet *Wallet) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// set wallet ID
	id := e.lastWalletID.Add(1)
	wallet.SetID(id)
	e.wallets[wallet.walletType] = append(e.wallets[wallet.walletType], wallet)
}
