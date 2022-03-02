package evilwallet

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"sync"
	"time"
)

const (
	GoFConfirmed             = 3
	waitForConfirmation      = 60 * time.Second
	FaucetRequestSplitNumber = 100
)

// region wallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

type EvilWallet interface {
	PrepareTransaction(es EvilScenario) (*ledgerstate.Transaction, error)
	PrepareDoubleSpendTransactions(es EvilScenario, numberOfSpends int, delays time.Duration) ([]*ledgerstate.Transaction, error)
}

type EvilScenario struct {
	// todo this should have instructions for evil wallet
	// how to handle this spamming scenario, which input wallet use,
	// where to store outputs of spam ect.
	// All logic of conflict creation will be hidden from spammer or integration test users
}

// Wallets functionality for users, to create test scenarios in user-friendly way
type Wallets struct {
	// keep track of outputs and corresponding wallets,
	// wallet type(faucet, spammer output without conflict, with conflicts)
	// track private keys
	wallets      []WalletInterface
	lastWalletID int // init with -1 value
	mu           sync.RWMutex
	// change statuses of outputs/transactions, wait for confirmations ect
	outputManager   *OutputManager
	conflictManager *ConflictManager
	connector       Clients
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// Faucet Requests /////////////////////////////////////////////////////////////////////////////////////////////////////

// AddWalletAndAssignID adds a new wallet to wallets and sets its ID with the next global numbering.
func (e *Wallets) AddWalletAndAssignID(wallet *Wallet) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// set wallet ID
	e.lastWalletID += 1
	wallet.SetID(e.lastWalletID)
	e.wallets = append(e.wallets, wallet)
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
			e.AddWalletAndAssignID(wallet)
		}(reqNum)
	}
	wg.Wait()
	return
}

// CreateFreshFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *Wallets) CreateFreshFaucetWallet() (w *Wallet, err error) {
	funds, err := e.requestAndSplitFaucetFunds()
	if err != nil {
		return
	}
	w = NewWallet()
	txIDs := e.splitOutputs(funds, w, FaucetRequestSplitNumber)

	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(w, txIDs, maxGoroutines)
	return
}

func (e *Wallets) requestAndSplitFaucetFunds() (wallet *Wallet, err error) {
	reqWallet := NewWallet()
	addr, idx := reqWallet.GenerateNextAddress()
	initOutput := e.requestFaucetFunds(addr)
	if err != nil {
		return
	}
	reqWallet.AddUnspentOutput(addr, idx, initOutput, uint64(faucet.Parameters.TokensPerRequest))
	// first split 1 to FaucetRequestSplitNumber outputs
	wallet = NewWallet()
	e.outputManager.AwaitWalletOutputsToBeConfirmed(reqWallet)
	txIDs := e.splitOutputs(reqWallet, wallet, FaucetRequestSplitNumber)
	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(wallet, txIDs, maxGoroutines)
	return
}

func (e *Wallets) requestFaucetFunds(addr ledgerstate.Address) (outputID ledgerstate.OutputID) {
	msgID := e.connector.RequestFaucetFunds(addr)
	if msgID == "" {
		return
	}
	outputStrID, ok := e.outputManager.AwaitUnspentOutputToBeConfirmed(addr, waitForConfirmation)
	if !ok {
		return
	}
	outputID, err := ledgerstate.OutputIDFromBase58(outputStrID)
	if err != nil {
		return
	}
	return
}

func (e *Wallets) splitOutputs(inputWallet *Wallet, outputWallet *Wallet, splitNumber int) []string {
	wg := sync.WaitGroup{}

	txIDs := make([]string, len(inputWallet.unspentOutputs))
	if inputWallet.unspentOutputs == nil {
		return []string{}
	}
	inputNum := 0
	for _, output := range inputWallet.unspentOutputs {
		wg.Add(1)
		go func(inputNum int, out *Output) {
			defer wg.Done()
			//tx := e.
			//tx := prepareTransaction([]*UnspentOutput{out}, []*seed.Seed{inputWallet.seed}, outWallet, numOutputs, *pledgeID)
			//clt := e.connector.GetClient()
			//txID, err := e.connector.PostTransaction(tx.Bytes(), clt)
			//if err != nil {
			//	return
			//}
			//txIDs[inputNum] = txID.TransactionID
		}(inputNum, output)
		inputNum++
	}
	wg.Wait()
	return txIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
