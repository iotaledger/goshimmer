package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet/sendfunds"
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
func (wallet *Wallet) SendFunds(options ...sendfunds.Option) (err error) {
	// build options from the parameters
	sendFundsOptions, err := sendfunds.BuildOptions(options...)
	if err != nil {
		return
	}

	// initialize default values
	if sendFundsOptions.RemainderAddress == address.Empty {
		sendFundsOptions.RemainderAddress = wallet.ReceiveAddress()
	}

	// aggregate total amount of required funds
	requiredFunds := make(map[balance.Color]uint64)
	for _, coloredBalances := range sendFundsOptions.Destinations {
		for color, amount := range coloredBalances {
			requiredFunds[color] += amount
		}
	}

	// look for the required balances in the unspent outputs
	outputsToUseAsInputs := make([]Output, 0)
	for _, unspentOutput := range wallet.UnspentOutputs() {
		requiredColorFoundInOutput := false
		for color, availableBalance := range unspentOutput.balances {
			if requiredAmount, requiredColorExists := requiredFunds[color]; requiredColorExists {
				if requiredAmount > availableBalance {
					requiredFunds[color] -= availableBalance
				} else {
					delete(requiredFunds, color)
				}

				requiredColorFoundInOutput = true
			}
		}

		if requiredColorFoundInOutput {
			outputsToUseAsInputs = append(outputsToUseAsInputs, unspentOutput)
		}
	}

	return
}

// ReceiveAddress returns the current receive address of the wallet
func (wallet *Wallet) ReceiveAddress() address.Address {
	return address.Empty
}

func (wallet *Wallet) UnspentOutputs(addresses ...address.Address) []Output {
	if len(addresses) >= 0 {
		// fill addresses with
	}

	return wallet.connector.UnspentOutputs(addresses...)
}

// Seed returns the seed of this wallet that is used to generate all of the wallets addresses and private keys.
func (wallet *Wallet) Seed() *Seed {
	return wallet.seed
}
