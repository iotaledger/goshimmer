package wallet

import (
	"errors"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/bitmask"
)

// Wallet represents a simple cryptocurrency wallet for the IOTA tangle. It contains the logic to manage the movement of
// funds.
type Wallet struct {
	addressManager *AddressManager
	connector      Connector

	// if this option is enabled the wallet will use a single reusable address instead of changing addresses.
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

	// initialize wallet with default address manager if we did not import a previous wallet
	if wallet.addressManager == nil {
		wallet.addressManager = NewAddressManager(NewSeed(), 0, []bitmask.BitMask{})
	}

	// initialize wallet with default connector (server) if none was provided
	if wallet.connector == nil {
		wallet.connector = &ServerConnector{}
	}

	return
}

// SendFunds issues a payment of the given amount to the given address.
func (wallet *Wallet) SendFunds(options ...SendFundsOption) (err error) {
	// build options from the parameters
	sendFundsOptions, err := BuildOptions(options...)
	if err != nil {
		return
	}

	// determine which outputs to use for our transfer
	consumedOutputs, err := wallet.determineOutputsToConsume(sendFundsOptions)
	if err != nil {
		return
	}

	// build transaction
	inputs, consumedFunds := wallet.buildInputs(consumedOutputs)
	outputs := wallet.buildOutputs(sendFundsOptions, consumedFunds)
	tx := transaction.New(inputs, outputs)
	for addr := range consumedOutputs {
		tx.Sign(signaturescheme.ED25519(*wallet.Seed().KeyPair(addr.Index)))
	}

	// mark addresses as spent
	if !wallet.singleAddress {
		for addr := range consumedOutputs {
			wallet.addressManager.MarkAddressSpent(addr.Index)
		}
	}

	// send transaction
	wallet.connector.SendTransaction(tx)

	return
}

// ReceiveAddress returns the last receive address of the wallet.
func (wallet *Wallet) ReceiveAddress() Address {
	return wallet.addressManager.LastUnspentAddress()
}

// NewReceiveAddress generates and returns a new unused receive address.
func (wallet *Wallet) NewReceiveAddress() Address {
	return wallet.addressManager.NewAddress()
}

// RemainderAddress returns the address that is used for the remainder of funds.
func (wallet *Wallet) RemainderAddress() Address {
	return wallet.addressManager.FirstUnspentAddress()
}

// UnspentOutputs returns the unspent outputs that are available for spending.
func (wallet *Wallet) UnspentOutputs() map[Address]map[transaction.ID]Output {
	return wallet.connector.UnspentOutputs(wallet.addressManager.Addresses()...)
}

// Seed returns the seed of this wallet that is used to generate all of the wallets addresses and private keys.
func (wallet *Wallet) Seed() *Seed {
	return wallet.addressManager.seed
}

func (wallet *Wallet) AddressManager() *AddressManager {
	return wallet.addressManager
}

func (wallet *Wallet) determineOutputsToConsume(sendFundsOptions *SendFundsOptions) (outputsToUseAsInputs map[Address]map[transaction.ID]Output, err error) {
	// initialize return values
	outputsToUseAsInputs = make(map[Address]map[transaction.ID]Output)

	// aggregate total amount of required funds, so we now what and how many funds we need
	requiredFunds := make(map[balance.Color]uint64)
	for _, coloredBalances := range sendFundsOptions.Destinations {
		for color, amount := range coloredBalances {
			requiredFunds[color] += amount
		}
	}

	// look for the required funds in the available unspent outputs
	for addr, unspentOutputsOnAddress := range wallet.UnspentOutputs() {
		// keeps track if outputs from this address are supposed to be spent
		outputsFromAddressSpent := false

		// scan the outputs on this address for required funds
		for transactionID, output := range unspentOutputsOnAddress {
			// keeps track if the output contains any usable funds
			requiredColorFoundInOutput := false

			// subtract the found matching balances from the required funds
			for color, availableBalance := range output.balances {
				if requiredAmount, requiredColorExists := requiredFunds[color]; requiredColorExists {
					if requiredAmount > availableBalance {
						requiredFunds[color] -= availableBalance
					} else {
						delete(requiredFunds, color)
					}

					requiredColorFoundInOutput = true
				}
			}

			// if we found required tokens in this output
			if requiredColorFoundInOutput {
				// store the output in the outputs to use for the transfer
				if _, addressEntryExists := outputsToUseAsInputs[addr]; !addressEntryExists {
					outputsToUseAsInputs[addr] = make(map[transaction.ID]Output)
				}
				outputsToUseAsInputs[addr][transactionID] = output

				// mark address as spent
				outputsFromAddressSpent = true
			}
		}

		// if outputs from this address were spent
		if outputsFromAddressSpent {
			// add the remaining outputs as well (we want to spend only once from every address)
			for transactionID, output := range unspentOutputsOnAddress {
				outputsToUseAsInputs[addr][transactionID] = output
			}
		}
	}

	// check if we have found all required funds
	if len(requiredFunds) != 0 {
		outputsToUseAsInputs = nil
		err = errors.New("not enough funds to create transaction")
	}

	return
}

func (wallet *Wallet) buildInputs(outputsToUseAsInputs map[Address]map[transaction.ID]Output) (inputs *transaction.Inputs, consumedFunds map[balance.Color]uint64) {
	consumedInputs := make([]transaction.OutputID, 0)
	consumedFunds = make(map[balance.Color]uint64)
	for addr, unspentOutputsOfAddress := range outputsToUseAsInputs {
		for transactionID, output := range unspentOutputsOfAddress {
			consumedInputs = append(consumedInputs, transaction.NewOutputID(addr.Address, transactionID))

			for color, amount := range output.balances {
				consumedFunds[color] += amount
			}
		}
	}
	inputs = transaction.NewInputs(consumedInputs...)

	return
}

func (wallet *Wallet) buildOutputs(sendFundsOptions *SendFundsOptions, consumedFunds map[balance.Color]uint64) (outputs *transaction.Outputs) {
	// build outputs for destinations
	outputsByColor := make(map[address.Address]map[balance.Color]uint64)
	for walletAddress, coloredBalances := range sendFundsOptions.Destinations {
		if _, addressExists := outputsByColor[walletAddress]; !addressExists {
			outputsByColor[walletAddress] = make(map[balance.Color]uint64)
		}
		for color, amount := range coloredBalances {
			outputsByColor[walletAddress][color] += amount
			consumedFunds[color] -= amount

			if consumedFunds[color] == 0 {
				delete(consumedFunds, color)
			}
		}
	}
	outputsBySlice := make(map[address.Address][]*balance.Balance)
	for addr, outputs := range outputsByColor {
		outputsBySlice[addr] = make([]*balance.Balance, 0)
		for color, amount := range outputs {
			outputsBySlice[addr] = append(outputsBySlice[addr], balance.New(color, int64(amount)))
		}
	}

	// update remainder address with default value (first unspent address) if none was provided
	if sendFundsOptions.RemainderAddress.Address == address.Empty {
		sendFundsOptions.RemainderAddress = wallet.RemainderAddress()
	}

	// build outputs for remainder
	if len(consumedFunds) != 0 {
		outputsBySlice[sendFundsOptions.RemainderAddress.Address] = make([]*balance.Balance, 0)

		for color, amount := range consumedFunds {
			outputsBySlice[sendFundsOptions.RemainderAddress.Address] = append(outputsBySlice[sendFundsOptions.RemainderAddress.Address], balance.New(color, int64(amount)))
		}
	}

	// construct result
	outputs = transaction.NewOutputs(outputsBySlice)

	return
}
