package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// Wallet represents a simple cryptocurrency wallet for the IOTA tangle. It contains the logic to manage the movement of
// funds.
type Wallet struct {
	seed          *Seed
	connector     Connector
	singleAddress bool
}

// New is the factory method of the wallet. It either creates a new wallet or restores the wallet backup that is handed
// in as an optional parameter.
func New(options ...Option) (wallet *Wallet) {
	// create wallet
	wallet = &Wallet{}

	// configure wallet
	for _, option := range options {
		option(wallet)
	}

	// initialize wallet with default seed if none was provided
	if wallet.seed == nil {
		wallet.seed = NewSeed()
	}

	// initialize wallet with default connector (server) if none was provided
	if wallet.connector == nil {
		wallet.connector = &ServerConnector{}
	}

	return
}

// SendFunds issues a payment of the given amount to the given address.
func (wallet *Wallet) SendFunds(address address.Address, amount uint64) (err error) {
	unspentOutputs := wallet.UnspentOutputs()

	return
}

// Seed returns the seed of this wallet that is used to generate all of the wallets addresses and private keys.
func (wallet *Wallet) Seed() *Seed {
	return wallet.seed
}
