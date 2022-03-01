package evilwallet

import (
	"errors"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const (
	GoFConfirmed        = 3
	waitForConfirmation = 60 * time.Second
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

func (e *EvilWallet) NewWallet(wType WalletType) *Wallet {
	return e.wallets.NewWallet(wType)
}

func (e *EvilWallet) GetClients(num int) []*client.GoShimmerAPI {
	return e.connector.GetClients(num)
}

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
		err = errors.New("output not confirmed!")
		return
	}

	return outputIDs[0], nil
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
