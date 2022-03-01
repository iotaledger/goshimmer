package evilwallet

import (
	"sync"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
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

// CreateNFreshFaucet10kWallet creates n new wallets, each wallet is created from one faucet request.
func (w *Wallets) CreateNFreshFaucet10kWallet(numberOf10kWallets int) {
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
			wallet, err := w.CreateFreshFaucetWallet()
			if err != nil {
				return
			}
			w.addWallet(wallet)
		}(reqNum)
	}
	wg.Wait()
	return
}

// CreateFreshFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (w *Wallets) CreateFreshFaucetWallet() (wallet *Wallet, err error) {
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
	index := uint64(w.lastAddrIdxUsed.Load())
	addr := w.seed.Address(index)
	w.indexAddrMap[index] = addr.Base58()
	w.lastAddrIdxUsed.Add(1)

	return addr
}
