package alias_wallet

import (
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
	"reflect"
	"time"
	"unsafe"
)

// alias wallet is a wallet that can handle aliases and extendedlockedoutputs

type Wallet struct {
	addressManager *AddressManager
	assetRegistry  *AssetRegistry
	outputManager  *OutputManager
	connector      Connector

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
	wallet.outputManager = NewUnspentOutputManager(wallet.addressManager, wallet.connector)
	err := wallet.outputManager.Refresh(true)
	if err != nil {
		panic(err)
	}

	return
}

// SendFunds sends funds from the wallet
func (wallet *Wallet) SendFunds(options ...SendFundsOption) (tx *ledgerstate.Transaction, err error) {
	// TODO: implement
	return
}

// CreateAsset creates a new colored token with the given details.
func (wallet *Wallet) CreateAsset(asset Asset) (assetColor ledgerstate.Color, err error) {
	// TODO: implement
	return
}

// DelegateFunds delegates funds to a given address by creating a delegated alias output.
func (wallet *Wallet) DelegateFunds() (tx *ledgerstate.Transaction, err error) {
	// TODO: implement
	return
}

// CreateNFT spends funds from the wallet to create an NFT.
func (wallet *Wallet) CreateNFT(options ...SendFundsOption) (tx *ledgerstate.Transaction, err error) { // build options from the parameters
	sendFundsOptions, err := buildSendFundsOptions(options...)
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

	// collect funds required for an alias input
	consumedOutputs, err := wallet.collectOutputsForAliasMint()
	// get a new address from address manager
	nftWalletAddress := wallet.NewReceiveAddress()
	// create the tx
	inputs := wallet.buildInputs(consumedOutputs)

	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	nft, err := ledgerstate.NewAliasOutputMint(
		map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: ledgerstate.DustThresholdAliasOutputIOTA},
		nftWalletAddress.Address(),
		nil,
	)
	unsortedOutputs := ledgerstate.Outputs{nft}

	if totalConsumedFunds[ledgerstate.ColorIOTA] == ledgerstate.DustThresholdAliasOutputIOTA {
		// all iota exhausted from consumed inputs
		delete(totalConsumedFunds, ledgerstate.ColorIOTA)
	} else {
		totalConsumedFunds[ledgerstate.ColorIOTA] -= ledgerstate.DustThresholdAliasOutputIOTA
	}
	remainderBalances := ledgerstate.NewColoredBalances(totalConsumedFunds)
	if remainderBalances.Size() != 0 {
		unsortedOutputs = append(unsortedOutputs, ledgerstate.NewSigLockedColoredOutput(
			remainderBalances, wallet.addressManager.FirstUnspentAddress().Address()))
	}
	outputs := ledgerstate.NewOutputs(unsortedOutputs...)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()
	inputsInOrder := ledgerstate.Outputs{}

	unlockBlocks := make([]ledgerstate.UnlockBlock, len(inputs))
	existingUnlockBlocks := make(map[address.Address]uint16)
	for outputIndex, input := range inputs {
		output := outputsByID[input.(*ledgerstate.UTXOInput).ReferencedOutputID()]
		inputsInOrder = append(inputsInOrder, output.Object)
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

	//check tx
	if !ledgerstate.TransactionBalancesValid(inputsInOrder, outputs) {
	}

	// mark outputs as spent
	for addr, outputs := range consumedOutputs {
		for outputID := range outputs {
			wallet.outputManager.MarkOutputSpent(addr, outputID)
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
func (wallet *Wallet) TransferNFT() {}
func (wallet *Wallet) DestroyNFT()  {}

// ServerStatus retrieves the connected server status.
func (wallet *Wallet) ServerStatus() (status ServerStatus, err error) {
	return wallet.connector.(*WebConnector).ServerStatus()
}

// AllowedPledgeNodeIDs retrieves the allowed pledge node IDs.
func (wallet *Wallet) AllowedPledgeNodeIDs() (res map[mana.Type][]string, err error) {
	return wallet.connector.(*WebConnector).GetAllowedPledgeIDs()
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
	return wallet.outputManager.UnspentOutputs()
}

// UnspentValueOutputs returns the unspent value type outputs that are available for spending.
func (wallet *Wallet) UnspentValueOutputs() map[address.Address]map[ledgerstate.OutputID]*Output {
	return wallet.outputManager.UnspentValueOutputs()
}

// UnspentAliasOutputs returns the unspent alias outputs that are available for spending.
func (wallet *Wallet) UnspentAliasOutputs() map[address.Address]map[ledgerstate.OutputID]*Output {
	return wallet.outputManager.UnspentAliasOutputs()
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
	err = wallet.outputManager.Refresh(rescanSpentAddresses...)

	return
}

// Balance returns the confirmed and pending balance of the funds managed by this wallet.
func (wallet *Wallet) Balance() (confirmedBalance map[ledgerstate.Color]uint64, pendingBalance map[ledgerstate.Color]uint64, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	confirmedBalance = make(map[ledgerstate.Color]uint64)
	pendingBalance = make(map[ledgerstate.Color]uint64)

	// iterate through the unspent outputs
	for addy, outputsOnAddress := range wallet.outputManager.UnspentValueOutputs() {
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

			switch output.Object.Type() {
			case ledgerstate.SigLockedSingleOutputType:
			case ledgerstate.SigLockedColoredOutputType:
				// extract balance
				output.Object.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
					targetMap[color] += balance
					return true
				})
			case ledgerstate.ExtendedLockedOutputType:
				casted := output.Object.(*ledgerstate.ExtendedLockedOutput)
				unlockAddyNow := casted.UnlockAddressNow(time.Now())
				if addy.Address().Equals(unlockAddyNow) {
					// we own this output now
					casted.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
						targetMap[color] += balance
						return true
					})
				}
			case ledgerstate.AliasOutputType:
				casted := output.Object.(*ledgerstate.AliasOutput)
				if casted.IsDelegated() {
					break
				}
				if casted.IsSelfGoverned() {
					// if it is self governed, addy is the state address, so we own everything
					casted.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
						targetMap[color] += balance
						return true
					})
					break
				}
				if casted.GetStateAddress().Equals(addy.Address()) {
					// we are state controller
					casted.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
						if color == ledgerstate.ColorIOTA {
							// the minimum amount can only be moved by the governor
							surplusIOTA := balance - ledgerstate.DustThresholdAliasOutputIOTA
							if surplusIOTA == 0 {
								return true
							}
							targetMap[color] += surplusIOTA
						} else {
							targetMap[color] += balance
						}
						return true
					})
					break
				}
				if casted.GetGoverningAddress().Equals(addy.Address()) {
					// we are the governor, so we only own the minimum dust amount that cannot be withdrawn by the state controller
					targetMap[ledgerstate.ColorIOTA] += ledgerstate.DustThresholdAliasOutputIOTA
					break
				}
			}
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

// collectOutputsForAliasMint tries to collect unspent outputs to fund minting an alias.
func (wallet *Wallet) collectOutputsForAliasMint() (OutputsByAddressAndOutputID, error) {
	requiredAmount := ledgerstate.DustThresholdAliasOutputIOTA
	_ = wallet.outputManager.Refresh()
	addresses := wallet.addressManager.Addresses()
	unspentOutputs := wallet.outputManager.UnspentValueOutputs(addresses...)

	var collected uint64
	outputsToConsume := NewAddressToOutputs()
	for i := wallet.addressManager.firstUnspentAddressIndex; i <= wallet.addressManager.lastAddressIndex; i++ {
		addy := wallet.addressManager.Address(i)
		for outputID, output := range unspentOutputs[addy] {
			amount, ok := output.Object.Balances().Get(ledgerstate.ColorIOTA)
			if ok {
				collected += amount
				// store the output in the outputs to use for the transfer
				if _, addressEntryExists := outputsToConsume[addy]; !addressEntryExists {
					outputsToConsume[addy] = make(map[ledgerstate.OutputID]*Output)
				}
				outputsToConsume[addy][outputID] = output

				if collected >= requiredAmount {
					return outputsToConsume, nil
				}
			}
		}
	}
	return nil, xerrors.Errorf("failed to gather %d IOTA funds, there are only %d IOTA funds available", requiredAmount, collected)
}

// buildInputs build a list of deterministically sorted inputs from the provided OutputsByAddressAndOutputID mapping.
func (wallet *Wallet) buildInputs(addressToIDToOutput OutputsByAddressAndOutputID) ledgerstate.Inputs {
	unsortedInputs := ledgerstate.Inputs{}
	for _, outputIDToOutputMap := range addressToIDToOutput {
		for _, output := range outputIDToOutputMap {
			unsortedInputs = append(unsortedInputs, output.Object.Input())
		}
	}
	return ledgerstate.NewInputs(unsortedInputs...)
}

// checkBalancesAndUnlocks checks if tx balances are okay and unlock blocks are valid.
func checkBalancesAndUnlocks(inputs ledgerstate.Outputs, tx *ledgerstate.Transaction) (bool, error) {
	balancesValid := ledgerstate.TransactionBalancesValid(inputs, tx.Essence().Outputs())
	unlocksValid, err := ledgerstate.UnlockBlocksValidWithError(inputs, tx)
	if err != nil {
		return false, err
	}
	return balancesValid && unlocksValid, nil
}
