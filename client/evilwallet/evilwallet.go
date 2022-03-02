package evilwallet

import (
	"errors"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const (
	GoFConfirmed             = 3
	waitForConfirmation      = 60 * time.Second
	FaucetRequestSplitNumber = 100

	maxGoroutines = 5
)

var clientsURL = []string{"http://localhost:8080", "http://localhost:8090"}

// region EvilWallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

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

func (e *EvilWallet) NewWallet(wType WalletType) *Wallet {
	return e.wallets.NewWallet(wType)
}

func (e *EvilWallet) GetClients(num int) []*client.GoShimmerAPI {
	return e.connector.GetClients(num)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// EvilWallet Faucet Requests /////////////////////////////////////////////////////////////////////////////////////////////////////

func (e *EvilWallet) RequestFundsFromFaucet(address string) (outputID ledgerstate.OutputID, err error) {
	// request funds from faucet
	err = e.connector.SendFaucetRequest(address)
	if err != nil {
		return
	}

	// track output in output manager
	outputIDs := e.outputManager.AddOutputsByAddress(address)
	allConfirmed := e.outputManager.Track(outputIDs)
	if !allConfirmed {
		err = errors.New("output not confirmed")
		return
	}

	return outputIDs[0], nil
}

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
			err := e.CreateFreshFaucetWallet()
			if err != nil {
				return
			}
		}(reqNum)
	}
	wg.Wait()
	return
}

// CreateFreshFaucetWallet creates a new wallet and fills the wallet with 10000 outputs created from funds
// requested from the Faucet.
func (e *EvilWallet) CreateFreshFaucetWallet() (err error) {
	funds, err := e.requestAndSplitFaucetFunds()
	if err != nil {
		return
	}
	w := e.NewWallet(fresh)
	txIDs := e.splitOutputs(funds, w, FaucetRequestSplitNumber)

	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(w, txIDs, maxGoroutines)
	return
}

func (e *EvilWallet) requestAndSplitFaucetFunds() (wallet *Wallet, err error) {
	reqWallet := NewWallet(fresh)
	addr := reqWallet.Address()
	initOutput := e.requestFaucetFunds(addr.Address())
	if err != nil {
		return
	}
	reqWallet.AddUnspentOutput(addr.Address(), initOutput, uint64(faucet.Parameters.TokensPerRequest))
	//first split 1 to FaucetRequestSplitNumber outputs
	wallet = NewWallet(fresh)
	e.outputManager.AwaitWalletOutputsToBeConfirmed(reqWallet)
	txIDs := e.splitOutputs(reqWallet, wallet, FaucetRequestSplitNumber)
	e.outputManager.AwaitTransactionsConfirmationAndUpdateWallet(wallet, txIDs, maxGoroutines)
	return
}

func (e *EvilWallet) requestFaucetFunds(addr ledgerstate.Address) (outputID ledgerstate.OutputID) {
	err := e.connector.SendFaucetRequest(addr.Base58())
	if err != nil {
		return
	}
	outputIDs := e.outputManager.AddOutputsByAddress(addr.Base58())
	ok := e.outputManager.Track(outputIDs)
	if !ok {
		return
	}
	outputID = outputIDs[0]
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
			// todo finish with a new create txs
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

// region EvilScenario ///////////////////////////////////////////////////////////////////////////////////////////////////////

type EvilScenario struct {
	// todo this should have instructions for evil wallet
	// how to handle this spamming scenario, which input wallet use,
	// where to store outputs of spam ect.
	// All logic of conflict creation will be hidden from spammer or integration test users
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
