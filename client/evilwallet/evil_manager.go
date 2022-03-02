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

const (
	maxGoroutines = 5
)

type WalletInterface interface {
	SplitBalanceEqually(splitNumber int, balance uint64) []uint64
	GenerateNextAddress() (ledgerstate.Address, uint64)
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

func (w *Wallet) SetID(id int) {
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

func (w *Wallet) SplitBalanceEqually(splitNumber int, balance uint64) []uint64 {
	outputBalances := make([]uint64, 0)
	// make sure the output balances are equal input
	var totalBalance uint64 = 0
	// input is divided equally among outputs
	for i := 0; i < splitNumber-1; i++ {
		outputBalances = append(outputBalances, balance/uint64(splitNumber))
		totalBalance, _ = ledgerstate.SafeAddUint64(totalBalance, outputBalances[i])
	}
	lastBalance, _ := ledgerstate.SafeSubUint64(balance, totalBalance)
	outputBalances = append(outputBalances, lastBalance)

	return outputBalances
}
