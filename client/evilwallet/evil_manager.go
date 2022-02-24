package evilwallet

import (
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
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

type EvilManager struct {
	wallets map[WalletType][]Wallet
	clients Clients

	lastWalletID atomic.Int64
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

func NewWallet(id int) *Wallet {
	idxSpent := atomic.NewInt64(-1)
	addrUsed := atomic.NewInt64(-1)
	return &Wallet{
		ID:                id,
		seed:              seed.NewSeed(),
		unspentOutputs:    make(map[string]*Output),
		indexAddrMap:      make(map[uint64]string),
		inputTransactions: make(map[string]types.Empty),
		lastAddrSpent:     *idxSpent,
		lastAddrIdxUsed:   *addrUsed,
		RWMutex:           &sync.RWMutex{},
	}
}

// CreateNFreshFaucet10kWallet creates n new wallets, each wallet is created from one faucet request.

func (e *EvilManager) CreateNFreshFaucet10kWallet(numberOf10kWallets int) {
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
func (e *EvilManager) CreateFreshFaucetWallet() (w *Wallet, err error) {
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

// addFreshWallet stores newly created wallet
func (e *EvilManager) addWallet(wallet *Wallet) {

}
