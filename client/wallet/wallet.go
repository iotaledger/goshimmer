package wallet

import (
	"errors"
	"reflect"
	"time"
	"unsafe"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
)

// Wallet represents a simple cryptocurrency wallet for the IOTA tangle. It contains the logic to manage the movement of
// funds.
type Wallet struct {
	addressManager       *AddressManager
	assetRegistry        *AssetRegistry
	unspentOutputManager *UnspentOutputManager
	connector            Connector

	// if this option is enabled the wallet will use a single reusable address instead of changing addresses.
	reusableAddress bool
}

// New is the factory method of the wallet. It either creates a new wallet or restores the wallet backup that is handed
// in as an optional parameter.
func New(options ...Option) (wallet *Wallet) {
	// create wallet
	wallet = &Wallet{
		assetRegistry: NewAssetRegistry(),
	}

	// configure wallet
	for _, option := range options {
		option(wallet)
	}

	// initialize wallet with default address manager if we did not import a previous wallet
	if wallet.addressManager == nil {
		wallet.addressManager = NewAddressManager(seed.NewSeed(), 0, []bitmask.BitMask{})
	}

	// initialize asset registry if none was provided in the options.
	if wallet.assetRegistry == nil {
		wallet.assetRegistry = NewAssetRegistry()
	}

	// initialize wallet with default connector (server) if none was provided
	if wallet.connector == nil {
		panic("you need to provide a connector for your wallet")
	}

	// initialize output manager
	wallet.unspentOutputManager = NewUnspentOutputManager(wallet.addressManager, wallet.connector)
	err := wallet.unspentOutputManager.Refresh(true)
	if err != nil {
		panic(err)
	}

	return
}

// ServerStatus retrieves the connected server status.
func (wallet *Wallet) ServerStatus() (status ServerStatus, err error) {
	return wallet.connector.(*WebConnector).ServerStatus()
}

// AllowedPledgeNodeIDs retrieves the allowed pledge node IDs.
func (wallet *Wallet) AllowedPledgeNodeIDs() (res map[mana.Type][]string, err error) {
	return wallet.connector.(*WebConnector).GetAllowedPledgeIDs()
}

// SendFunds issues a payment of the given amount to the given address.
func (wallet *Wallet) SendFunds(options ...SendFundsOption) (tx *ledgerstate.Transaction, err error) {
	// build options from the parameters
	sendFundsOptions, err := buildSendFundsOptions(options...)
	if err != nil {
		return
	}

	// determine which outputs to use for our transfer
	consumedOutputs, err := wallet.determineOutputsToConsume(sendFundsOptions)
	if err != nil {
		return
	}

	// determine pledge IDs
	allowedPledgeNodeIDs, err := wallet.connector.GetAllowedPledgeIDs()
	if err != nil {
		return
	}
	var accessPledgeNodeID identity.ID
	if sendFundsOptions.AccessManaPledgeID == "" {
		accessPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		accessPledgeNodeID, err = mana.IDFromStr(sendFundsOptions.AccessManaPledgeID)
	}
	if err != nil {
		return
	}

	var consensusPledgeNodeID identity.ID
	if sendFundsOptions.ConsensusManaPledgeID == "" {
		consensusPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		consensusPledgeNodeID, err = mana.IDFromStr(sendFundsOptions.ConsensusManaPledgeID)
	}
	if err != nil {
		return
	}

	// build transaction
	inputs, consumedFunds := wallet.buildInputs(consumedOutputs)
	outputs := wallet.buildOutputs(sendFundsOptions, consumedFunds)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()

	unlockBlocks := make([]ledgerstate.UnlockBlock, len(inputs))
	existingUnlockBlocks := make(map[address.Address]uint16)
	for outputIndex, input := range inputs {
		output := outputsByID[input.(*ledgerstate.UTXOInput).ReferencedOutputID()]
		if unlockBlockIndex, unlockBlockExists := existingUnlockBlocks[output.Address]; unlockBlockExists {
			unlockBlocks[outputIndex] = ledgerstate.NewReferenceUnlockBlock(unlockBlockIndex)
			continue
		}

		keyPair := wallet.Seed().KeyPair(output.Address.Index)
		unlockBlock := ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(txEssence.Bytes())))
		unlockBlocks[outputIndex] = unlockBlock
		existingUnlockBlocks[output.Address] = uint16(len(existingUnlockBlocks))
	}

	tx = ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// mark outputs as spent
	for addr, outputs := range consumedOutputs {
		for transactionID := range outputs {
			wallet.unspentOutputManager.MarkOutputSpent(addr, transactionID)
		}
	}

	// mark addresses as spent
	if !wallet.reusableAddress {
		for addr := range consumedOutputs {
			wallet.addressManager.MarkAddressSpent(addr.Index)
		}
	}

	// send transaction
	err = wallet.connector.SendTransaction(tx)

	return
}

// CreateAsset creates a new colored token with the given details.
func (wallet *Wallet) CreateAsset(asset Asset) (assetColor ledgerstate.Color, err error) {
	if asset.Amount == 0 {
		err = errors.New("required to provide the amount when trying to create an asset")

		return
	}

	if asset.Name == "" {
		err = errors.New("required to provide a name when trying to create an asset")

		return
	}

	tx, err := wallet.SendFunds(
		Destination(wallet.ReceiveAddress(), asset.Amount, ledgerstate.ColorMint),
	)
	if err != nil {
		return
	}

	// this only works if there is only one MINT output in the transaction
	assetColor = ledgerstate.ColorIOTA
	for _, output := range tx.Essence().Outputs() {
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if color == ledgerstate.ColorMint {
				digest := blake2b.Sum256(output.ID().Bytes())
				assetColor, _, err = ledgerstate.ColorFromBytes(digest[:])
			}
			return true
		})
	}

	if err != nil {
		return
	}

	if assetColor != ledgerstate.ColorIOTA {
		wallet.assetRegistry.RegisterAsset(assetColor, asset)
	}

	return
}

// AssetRegistry return the internal AssetRegistry instance of the wallet.
func (wallet *Wallet) AssetRegistry() *AssetRegistry {
	return wallet.assetRegistry
}

// ReceiveAddress returns the last receive address of the wallet.
func (wallet *Wallet) ReceiveAddress() address.Address {
	return wallet.addressManager.LastUnspentAddress()
}

// NewReceiveAddress generates and returns a new unused receive address.
func (wallet *Wallet) NewReceiveAddress() address.Address {
	return wallet.addressManager.NewAddress()
}

// RemainderAddress returns the address that is used for the remainder of funds.
func (wallet *Wallet) RemainderAddress() address.Address {
	return wallet.addressManager.FirstUnspentAddress()
}

// UnspentOutputs returns the unspent outputs that are available for spending.
func (wallet *Wallet) UnspentOutputs() map[address.Address]map[ledgerstate.OutputID]*Output {
	return wallet.unspentOutputManager.UnspentOutputs()
}

// RequestFaucetFunds requests some funds from the faucet for testing purposes.
func (wallet *Wallet) RequestFaucetFunds(waitForConfirmation ...bool) (err error) {
	if len(waitForConfirmation) == 0 || !waitForConfirmation[0] {
		err = wallet.connector.RequestFaucetFunds(wallet.ReceiveAddress())

		return
	}

	if err = wallet.Refresh(); err != nil {
		return
	}
	confirmedBalance, _, err := wallet.Balance()
	if err != nil {
		return
	}

	err = wallet.connector.RequestFaucetFunds(wallet.ReceiveAddress())
	if err != nil {
		return
	}

	for {
		time.Sleep(500 * time.Millisecond)

		if err = wallet.Refresh(); err != nil {
			return
		}
		newConfirmedBalance, _, balanceErr := wallet.Balance()
		if balanceErr != nil {
			err = balanceErr

			return
		}

		if !reflect.DeepEqual(confirmedBalance, newConfirmedBalance) {
			return
		}
	}
}

// Refresh scans the addresses for incoming transactions. If the optional rescanSpentAddresses parameter is set to true
// we also scan the spent addresses again (this can take longer).
func (wallet *Wallet) Refresh(rescanSpentAddresses ...bool) (err error) {
	err = wallet.unspentOutputManager.Refresh(rescanSpentAddresses...)

	return
}

// Balance returns the confirmed and pending balance of the funds managed by this wallet.
func (wallet *Wallet) Balance() (confirmedBalance map[ledgerstate.Color]uint64, pendingBalance map[ledgerstate.Color]uint64, err error) {
	err = wallet.unspentOutputManager.Refresh()
	if err != nil {
		return
	}

	confirmedBalance = make(map[ledgerstate.Color]uint64)
	pendingBalance = make(map[ledgerstate.Color]uint64)

	// iterate through the unspent outputs
	for _, outputsOnAddress := range wallet.unspentOutputManager.UnspentOutputs() {
		for _, output := range outputsOnAddress {
			// skip if the output was rejected or spent already
			if output.InclusionState.Spent || output.InclusionState.Rejected {
				continue
			}

			// determine target map
			var targetMap map[ledgerstate.Color]uint64
			if output.InclusionState.Confirmed {
				targetMap = confirmedBalance
			} else {
				targetMap = pendingBalance
			}

			// store amount
			output.Balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				targetMap[color] += balance
				return true
			})
		}
	}

	return
}

// Seed returns the seed of this wallet that is used to generate all of the wallets addresses and private keys.
func (wallet *Wallet) Seed() *seed.Seed {
	return wallet.addressManager.seed
}

// AddressManager returns the manager for the addresses of this wallet.
func (wallet *Wallet) AddressManager() *AddressManager {
	return wallet.addressManager
}

// ExportState exports the current state of the wallet to a marshaled version.
func (wallet *Wallet) ExportState() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteBytes(wallet.Seed().Bytes())
	marshalUtil.WriteUint64(wallet.AddressManager().lastAddressIndex)
	marshalUtil.WriteBytes(wallet.assetRegistry.Bytes())
	marshalUtil.WriteBytes(*(*[]byte)(unsafe.Pointer(&wallet.addressManager.spentAddresses)))

	return marshalUtil.Bytes()
}

func (wallet *Wallet) determineOutputsToConsume(sendFundsOptions *sendFundsOptions) (outputsToConsume OutputsByAddressAndOutputID, err error) {
	// initialize return values
	outputsToConsume = make(OutputsByAddressAndOutputID)

	// aggregate total amount of required funds, so we now what and how many funds we need
	requiredFunds := make(map[ledgerstate.Color]uint64)
	for _, coloredBalances := range sendFundsOptions.Destinations {
		for color, amount := range coloredBalances {
			// if we want to color sth then we need fresh IOTA
			if color == ledgerstate.ColorMint {
				color = ledgerstate.ColorIOTA
			}

			requiredFunds[color] += amount
		}
	}

	// refresh balances so we get the latest changes
	if err = wallet.unspentOutputManager.Refresh(); err != nil {
		return
	}

	// look for the required funds in the available unspent outputs
	for addr, unspentOutputsOnAddress := range wallet.unspentOutputManager.UnspentOutputs() {
		// keeps track if outputs from this address are supposed to be spent
		outputsFromAddressSpent := false

		// scan the outputs on this address for required funds
		for outputID, output := range unspentOutputsOnAddress {
			// keeps track if the output contains any usable funds
			requiredColorFoundInOutput := false

			// subtract the found matching balances from the required funds
			output.Balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if requiredAmount, requiredColorExists := requiredFunds[color]; requiredColorExists {
					if requiredAmount > balance {
						requiredFunds[color] -= balance
					} else {
						delete(requiredFunds, color)
					}

					requiredColorFoundInOutput = true
				}
				return true
			})

			// if we found required tokens in this output
			if requiredColorFoundInOutput {
				// store the output in the outputs to use for the transfer
				if _, addressEntryExists := outputsToConsume[addr]; !addressEntryExists {
					outputsToConsume[addr] = make(map[ledgerstate.OutputID]*Output)
				}
				outputsToConsume[addr][outputID] = output

				// mark address as spent
				outputsFromAddressSpent = true
			}
		}

		// if outputs from this address were spent add the remaining outputs as well (we want to spend only once from
		// every address if we are not using a reusable address)
		if !wallet.reusableAddress && outputsFromAddressSpent {
			for transactionID, output := range unspentOutputsOnAddress {
				outputsToConsume[addr][transactionID] = output
			}
		}
	}

	// update remainder address with default value (first unspent address) if none was provided
	if sendFundsOptions.RemainderAddress == address.AddressEmpty {
		sendFundsOptions.RemainderAddress = wallet.RemainderAddress()
	}
	if _, remainderAddressInConsumedOutputs := outputsToConsume[sendFundsOptions.RemainderAddress]; remainderAddressInConsumedOutputs && !wallet.reusableAddress {
		sendFundsOptions.RemainderAddress = wallet.ReceiveAddress()
	}
	if _, remainderAddressInConsumedOutputs := outputsToConsume[sendFundsOptions.RemainderAddress]; remainderAddressInConsumedOutputs && !wallet.reusableAddress {
		sendFundsOptions.RemainderAddress = wallet.NewReceiveAddress()
	}

	// check if we have found all required funds
	if len(requiredFunds) != 0 {
		outputsToConsume = nil
		err = errors.New("not enough funds to create transaction")
	}

	return
}

func (wallet *Wallet) buildInputs(outputsToUseAsInputs map[address.Address]map[ledgerstate.OutputID]*Output) (inputs ledgerstate.Inputs, consumedFunds map[ledgerstate.Color]uint64) {
	consumedInputs := make(ledgerstate.Inputs, 0)
	consumedFunds = make(map[ledgerstate.Color]uint64)
	for _, unspentOutputsOfAddress := range outputsToUseAsInputs {
		i := uint16(0)
		for outputID, output := range unspentOutputsOfAddress {
			input := ledgerstate.NewUTXOInput(outputID)
			consumedInputs = append(consumedInputs, input)
			i++
			output.Balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				consumedFunds[color] += balance
				return true
			})
		}
	}
	inputs = ledgerstate.NewInputs(consumedInputs...)
	return
}

func (wallet *Wallet) buildOutputs(sendFundsOptions *sendFundsOptions, consumedFunds map[ledgerstate.Color]uint64) (outputs ledgerstate.Outputs) {
	// build outputs for destinations
	outputsByColor := make(map[address.Address]map[ledgerstate.Color]uint64)
	for walletAddress, coloredBalances := range sendFundsOptions.Destinations {
		if _, addressExists := outputsByColor[walletAddress]; !addressExists {
			outputsByColor[walletAddress] = make(map[ledgerstate.Color]uint64)
		}
		for color, amount := range coloredBalances {
			outputsByColor[walletAddress][color] += amount
			if color == ledgerstate.ColorMint {
				consumedFunds[ledgerstate.ColorIOTA] -= amount

				if consumedFunds[ledgerstate.ColorIOTA] == 0 {
					delete(consumedFunds, ledgerstate.ColorIOTA)
				}
			} else {
				consumedFunds[color] -= amount

				if consumedFunds[color] == 0 {
					delete(consumedFunds, color)
				}
			}
		}
	}

	// build outputs for remainder
	if len(consumedFunds) != 0 {
		if _, addressExists := outputsByColor[sendFundsOptions.RemainderAddress]; !addressExists {
			outputsByColor[sendFundsOptions.RemainderAddress] = make(map[ledgerstate.Color]uint64)
		}

		for color, amount := range consumedFunds {
			outputsByColor[sendFundsOptions.RemainderAddress][color] += amount
		}
	}

	// construct result
	var outputsSlice []ledgerstate.Output
	for addr, outputs := range outputsByColor {
		coloredBalances := ledgerstate.NewColoredBalances(outputs)
		output := ledgerstate.NewSigLockedColoredOutput(coloredBalances, addr.Address())
		outputsSlice = append(outputsSlice, output)
	}
	outputs = ledgerstate.NewOutputs(outputsSlice...)

	return
}
