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

	maxGoroutines = 5
)

var clientsURL = []string{"http://localhost:8080", "http://localhost:8090"}

// region Evilwallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet provides a user-friendly way to do complicated double spend scenarios.
type EvilWallet struct {
	wallets         *Wallets
	connector       Clients
	outputManager   *OutputManager
	conflictManager *ConflictManager
}

// NewEvilWallet creates an EvilWallet instance.
func NewEvilWallet() *EvilWallet {
	connector := NewConnector(clientsURL)

	return &EvilWallet{
		wallets:         NewWallets(),
		connector:       connector,
		outputManager:   NewOutputManager(connector),
		conflictManager: NewConflictManager(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet Faucet Requests /////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateNFreshFaucet10kWallet creates n new wallets, each wallet is created from one faucet request.
func (e *EvilWallet) CreateNFreshFaucet10kWallet(numberOf10kWallets int) {
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
			e.wallets.AddWalletAndAssignID(wallet)
		}(reqNum)
	}
	wg.Wait()
	return
}

// CreateFreshFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) CreateFreshFaucetWallet() (w *Wallet, err error) {
	funds, err := e.requestAndSplitFaucetFunds()
	if err != nil {
		return
	}
	w = NewWallet()
	txIDs := e.splitOutputs(funds, w, FaucetRequestSplitNumber)

	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(w, txIDs, maxGoroutines)
	return
}

func (e *EvilWallet) requestAndSplitFaucetFunds() (wallet *Wallet, err error) {
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

func (e *EvilWallet) requestFaucetFunds(addr ledgerstate.Address) (outputID ledgerstate.OutputID) {
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

func (e *EvilWallet) splitOutputs(inputWallet *Wallet, outputWallet *Wallet, splitNumber int) []string {
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

// region Evilwallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

type EvilScenario struct {
	// todo this should have instructions for evil wallet
	// how to handle this spamming scenario, which input wallet use,
	// where to store outputs of spam ect.
	// All logic of conflict creation will be hidden from spammer or integration test users
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
