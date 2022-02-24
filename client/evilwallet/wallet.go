package evilwallet

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"time"
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
	evilManager *EvilManager
	// change statuses of outputs/transactions, wait for confirmations ect
	outputManager   *OutputManager
	conflictManager *ConflictManager
	connector       Clients
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
