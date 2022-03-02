package evilwallet

import (
	"sync"

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

const (
	maxGoroutines = 5
)


type Wallets struct {
	wallets map[WalletType][]*Wallet

	clients Clients

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


// CreateNFreshFaucet10kWallet creates n new wallets, each wallet is created from one faucet request.
func (e *Wallets) CreateNFreshFaucet10kWallet(numberOf10kWallets int) {
	// channel to block the number of concurrent goroutines
	semaphore := make(chan bool, maxGoroutines)
	wg := sync.WaitGroup{}

	for reqNum := 0; reqNum < numberOf10kWallets; reqNum++ {
		wg.Add(1)
		go func(reqNum int) {
			defer wg.Done()
			// block and release goroutines
			semaphore <- true
			defer func() {
				<-semaphore
			}()
			wallet, err := e.CreateFreshFaucetWallet()
			if err != nil {
				return
			}
			e.addWallet(wallet)
		}(reqNum)
	}
	wg.Wait()
	return
}


// CreateFreshFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *Wallets) CreateFreshFaucetWallet() (w *Wallet, err error) {
	//clt := e.clients.GetClient()
	//funds, err := requestAndSplitFaucetFunds()
	//if err != nil {
	//	return
	//}
	//w := NewWallet(e.GenerateWalletID())

	//txIDs := splitOutputs(funds, w, 100)

	// todo here we could trigger awaitFaucetWallet and output manager would do the rest
	//e.AwaitTransactionsToBeConfirmed(w, txIDs, clt, 5)
	return
}

func requestAndSplitFaucetFunds() (wallet *Wallet, err error) {
	return
}


func SplitBalanceEqually(splitNumber int, balance uint64) []uint64 {
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
