package wallet

import (
	"context"
	"reflect"
	"time"
	"unsafe"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/bitmask"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/claimconditionaloptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/consolidateoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/createnftoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/deposittonftoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/destroynftoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownednftsoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownedoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/transfernftoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/withdrawfromnftoptions"
	mana2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/congestioncontrol/icca/mana"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/utxo"
	devnetvm2 "github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/vm/devnetvm"
)

// region Wallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// DefaultPollingInterval is the polling interval of the wallet when waiting for confirmation (in ms).
	DefaultPollingInterval = 500 * time.Millisecond
	// DefaultConfirmationTimeout is the timeout of waiting for confirmation. (in ms).
	DefaultConfirmationTimeout = 150000 * time.Millisecond
	// DefaultAssetRegistryNetwork is the default asset registry network.
	DefaultAssetRegistryNetwork = "nectar"
)

// ErrTooManyOutputs is an error returned when the number of outputs/inputs exceeds the protocol wide constant.
var ErrTooManyOutputs = errors.New("number of outputs is more, than supported for a single transaction")

// Wallet is a wallet that can handle aliases and extendedlockedoutputs.
type Wallet struct {
	addressManager *AddressManager
	assetRegistry  *AssetRegistry
	outputManager  *OutputManager
	connector      Connector

	faucetPowDifficulty int
	// if this option is enabled the wallet will use a single reusable address instead of changing addresses.
	reusableAddress          bool
	ConfirmationPollInterval time.Duration
	ConfirmationTimeout      time.Duration
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

	if wallet.ConfirmationPollInterval == 0 {
		wallet.ConfirmationPollInterval = DefaultPollingInterval
	}

	if wallet.ConfirmationTimeout == 0 {
		wallet.ConfirmationTimeout = DefaultConfirmationTimeout
	}

	// initialize wallet with default address manager if we did not import a previous wallet
	if wallet.addressManager == nil {
		wallet.addressManager = NewAddressManager(seed.NewSeed(), 0, []bitmask.BitMask{})
	}

	// initialize asset registry if none was provided in the options.
	if wallet.assetRegistry == nil {
		wallet.assetRegistry = NewAssetRegistry(DefaultAssetRegistryNetwork)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SendFunds ////////////////////////////////////////////////////////////////////////////////////////////////////

// SendFunds sends funds from the wallet.
func (wallet *Wallet) SendFunds(options ...sendoptions.SendFundsOption) (tx *devnetvm2.Transaction, err error) {
	sendOptions, err := sendoptions.Build(options...)
	if err != nil {
		return
	}

	// how much funds will we need to fund this transfer?
	requiredFunds := sendOptions.RequiredFunds()
	// collect that many outputs for funding
	consumedOutputs, err := wallet.collectOutputsForFunding(requiredFunds, sendOptions.UsePendingOutputs, sendOptions.SourceAddresses...)
	if err != nil {
		if errors.Is(err, ErrTooManyOutputs) {
			err = errors.Errorf("consolidate funds and try again: %w", err)
		}
		return
	}

	// determine pledgeIDs
	aPledgeID, cPledgeID, err := wallet.derivePledgeIDs(sendOptions.AccessManaPledgeID, sendOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	// build inputs from consumed outputs
	inputs := wallet.buildInputs(consumedOutputs)
	// aggregate all the funds we consume from inputs
	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	remainderAddress := wallet.chooseRemainderAddress(consumedOutputs, sendOptions.RemainderAddress)
	outputs := wallet.buildOutputs(sendOptions, totalConsumedFunds, remainderAddress)

	txEssence := devnetvm2.NewTransactionEssence(0, time.Now(), aPledgeID, cPledgeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()

	unlockBlocks, inputsAsOutputsInOrder := wallet.buildUnlockBlocks(inputs, outputsByID, txEssence)

	tx = devnetvm2.NewTransaction(txEssence, unlockBlocks)
	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	// check syntactical validity by marshaling an unmarshalling
	tx = new(devnetvm2.Transaction)
	err = tx.FromBytes(txBytes)
	if err != nil {
		return nil, err
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsAsOutputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(consumedOutputs)

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}
	if sendOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID(), sendOptions.Context)
	}

	return tx, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsolidateFunds /////////////////////////////////////////////////////////////////////////////////////////////

// ConsolidateFunds consolidates available wallet funds into one output.
func (wallet *Wallet) ConsolidateFunds(options ...consolidateoptions.ConsolidateFundsOption) (txs []*devnetvm2.Transaction, err error) {
	consolidateOptions, err := consolidateoptions.Build(options...)
	if err != nil {
		return
	}
	// get available balances
	confirmedAvailableBalance, _, err := wallet.AvailableBalance()
	if err != nil {
		return
	}
	if len(confirmedAvailableBalance) == 0 {
		err = errors.Errorf("no available balance to be consolidated in wallet")
		return
	}
	// collect outputs
	allOutputs, err := wallet.collectOutputsForFunding(confirmedAvailableBalance, false)
	if err != nil && !errors.Is(err, ErrTooManyOutputs) {
		return
	}
	if allOutputs.OutputCount() == 1 {
		err = errors.Errorf("can't consolidate funds, there is only one value output in wallet")
		return
	}
	consumedOutputsSlice := allOutputs.SplitIntoChunksOfMaxInputCount()

	for _, consumedOutputs := range consumedOutputsSlice {
		// build inputs from consumed outputs
		inputs := wallet.buildInputs(consumedOutputs)
		// aggregate all the funds we consume from inputs
		totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
		toAddress := wallet.chooseToAddress(consumedOutputs, address.AddressEmpty) // no optional toAddress from options

		outputs := devnetvm2.NewOutputs(devnetvm2.NewSigLockedColoredOutput(devnetvm2.NewColoredBalances(totalConsumedFunds), toAddress.Address()))

		// determine pledgeIDs
		aPledgeID, cPledgeID, pErr := wallet.derivePledgeIDs(consolidateOptions.AccessManaPledgeID, consolidateOptions.ConsensusManaPledgeID)
		if pErr != nil {
			err = pErr
			return
		}

		txEssence := devnetvm2.NewTransactionEssence(0, time.Now(), aPledgeID, cPledgeID, inputs, outputs)
		outputsByID := consumedOutputs.OutputsByID()

		unlockBlocks, inputsAsOutputsInOrder := wallet.buildUnlockBlocks(inputs, outputsByID, txEssence)

		tx := devnetvm2.NewTransaction(txEssence, unlockBlocks)

		txBytes, err := tx.Bytes()
		if err != nil {
			return nil, err
		}
		// check syntactical validity by marshaling an unmarshalling
		tx = new(devnetvm2.Transaction)
		err = tx.FromBytes(txBytes)
		if err != nil {
			return nil, err
		}
		// check tx validity (balances, unlock blocks)
		ok, cErr := checkBalancesAndUnlocks(inputsAsOutputsInOrder, tx)
		if cErr != nil {
			return nil, cErr
		}
		if !ok {
			return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
		}

		wallet.markOutputsAndAddressesSpent(consumedOutputs)
		err = wallet.connector.SendTransaction(tx)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
		if consolidateOptions.WaitForConfirmation {
			err = wallet.WaitForTxAcceptance(tx.ID())
			if err != nil {
				return txs, err
			}
		}
	}

	return txs, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ClaimConditionalFunds ////////////////////////////////////////////////////////////////////////////////////////

// ClaimConditionalFunds gathers all currently conditionally owned outputs and consolidates them into the output.
func (wallet *Wallet) ClaimConditionalFunds(options ...claimconditionaloptions.ClaimConditionalFundsOption) (tx *devnetvm2.Transaction, err error) {
	claimOptions, err := claimconditionaloptions.Build(options...)
	if err != nil {
		return
	}
	confirmedConditionalBalance, _, err := wallet.ConditionalBalances()
	if err != nil {
		return
	}
	if len(confirmedConditionalBalance) == 0 {
		err = errors.Errorf("no conditional balance found in the wallet")
		return
	}
	addresses := wallet.addressManager.Addresses()
	consumedOutputs := wallet.outputManager.UnspentConditionalOutputs(false, addresses...)
	if len(consumedOutputs) == 0 {
		err = errors.Errorf("failed to find conditionally owned outputs in wallet")
		return
	}

	// build inputs from consumed outputs
	inputs := wallet.buildInputs(consumedOutputs)
	// aggregate all the funds we consume from inputs
	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	toAddress := wallet.chooseToAddress(consumedOutputs, address.AddressEmpty) // no optional toAddress from options
	outputs := devnetvm2.NewOutputs(devnetvm2.NewSigLockedColoredOutput(devnetvm2.NewColoredBalances(totalConsumedFunds), toAddress.Address()))

	// determine pledgeIDs
	aPledgeID, cPledgeID, err := wallet.derivePledgeIDs(claimOptions.AccessManaPledgeID, claimOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	txEssence := devnetvm2.NewTransactionEssence(0, time.Now(), aPledgeID, cPledgeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()

	unlockBlocks, inputsAsOutputsInOrder := wallet.buildUnlockBlocks(inputs, outputsByID, txEssence)

	tx = devnetvm2.NewTransaction(txEssence, unlockBlocks)

	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	// check syntactical validity by marshaling an unmarshalling
	tx = new(devnetvm2.Transaction)
	err = tx.FromBytes(txBytes)
	if err != nil {
		return nil, err
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsAsOutputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(consumedOutputs)

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}
	if claimOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}
	return
}

// endregion //////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CreateAsset //////////////////////////////////////////////////////////////////////////////////////////////////

// CreateAsset creates a new colored token with the given details.
func (wallet *Wallet) CreateAsset(asset Asset, waitForConfirmation ...bool) (assetColor devnetvm2.Color, err error) {
	if asset.Supply == 0 {
		err = errors.New("required to provide the amount when trying to create an asset")

		return
	}

	if asset.Name == "" {
		err = errors.New("required to provide a name when trying to create an asset")

		return
	}

	// where will we spend from?
	consumedOutputs, err := wallet.collectOutputsForFunding(map[devnetvm2.Color]uint64{devnetvm2.ColorIOTA: asset.Supply}, false)
	if err != nil {
		if errors.Is(err, ErrTooManyOutputs) {
			err = errors.Errorf("consolidate funds and try again: %w", err)
		}
		return
	}
	receiveAddress := wallet.chooseToAddress(consumedOutputs, address.AddressEmpty)

	var wait bool
	if len(waitForConfirmation) > 0 {
		wait = waitForConfirmation[0]
	}

	tx, err := wallet.SendFunds(
		sendoptions.Destination(receiveAddress, asset.Supply, devnetvm2.ColorMint),
		sendoptions.WaitForConfirmation(wait),
		sendoptions.UsePendingOutputs(false),
	)
	if err != nil {
		return
	}

	// this only works if there is only one MINT output in the transaction
	assetColor = devnetvm2.ColorIOTA
	for _, output := range tx.Essence().Outputs() {
		output.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
			if color == devnetvm2.ColorMint {
				digest := blake2b.Sum256(output.ID().Bytes())
				assetColor, _, err = devnetvm2.ColorFromBytes(digest[:])
			}
			return true
		})
	}

	if err != nil {
		return
	}

	if assetColor != devnetvm2.ColorIOTA {
		asset.Color = assetColor
		asset.TransactionID = tx.ID()
		wallet.assetRegistry.RegisterAsset(assetColor, asset)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CreateNFT ////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateNFT spends funds from the wallet to create an NFT.
func (wallet *Wallet) CreateNFT(options ...createnftoptions.CreateNFTOption) (tx *devnetvm2.Transaction, nftID *devnetvm2.AliasAddress, err error) { // build options from the parameters
	// build options
	createNFTOptions, err := createnftoptions.Build(options...)
	if err != nil {
		return
	}
	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(createNFTOptions.AccessManaPledgeID, createNFTOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}
	// collect funds required for an alias input
	consumedOutputs, err := wallet.collectOutputsForFunding(createNFTOptions.InitialBalance, false)
	if err != nil {
		if errors.Is(err, ErrTooManyOutputs) {
			err = errors.Errorf("consolidate funds and try again: %w", err)
		}
		return nil, nil, err
	}
	// determine which address should receive the nft
	nftWalletAddress := wallet.chooseToAddress(consumedOutputs, address.AddressEmpty)
	// build inputs from consumed outputs
	inputs := wallet.buildInputs(consumedOutputs)
	// aggregate all the funds we consume from inputs
	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	// create an alias mint output
	nft, err := devnetvm2.NewAliasOutputMint(
		createNFTOptions.InitialBalance,
		nftWalletAddress.Address(),
		createNFTOptions.ImmutableData,
	)
	if err != nil {
		return nil, nil, err
	}
	unsortedOutputs := devnetvm2.Outputs{nft}

	// calculate remainder balances (consumed - nft balance)
	nft.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
		totalConsumedFunds[color] -= balance
		if totalConsumedFunds[color] <= 0 {
			delete(totalConsumedFunds, color)
		}
		return true
	})
	remainderBalances := devnetvm2.NewColoredBalances(totalConsumedFunds)
	// only add remainder output if there is a remainder balance
	if remainderBalances.Size() != 0 {
		unsortedOutputs = append(unsortedOutputs, devnetvm2.NewSigLockedColoredOutput(
			remainderBalances, wallet.chooseRemainderAddress(consumedOutputs, address.AddressEmpty).Address()))
	}
	// create tx essence
	outputs := devnetvm2.NewOutputs(unsortedOutputs...)
	txEssence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, inputs, outputs)

	// build unlock blocks
	unlockBlocks, inputsInOrder := wallet.buildUnlockBlocks(inputs, consumedOutputs.OutputsByID(), txEssence)

	tx = devnetvm2.NewTransaction(txEssence, unlockBlocks)

	txBytes, err := tx.Bytes()
	if err != nil {
		return
	}
	// check syntactical validity by marshaling an unmarshalling
	tx = new(devnetvm2.Transaction)
	err = tx.FromBytes(txBytes)
	if err != nil {
		return
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	// look for the id of the freshly created nft (alias) that is only available after the outputID is set.
	for _, output := range tx.Essence().Outputs() {
		if output.Type() == devnetvm2.AliasOutputType {
			// Address() for an alias output returns the alias address, the unique ID of the alias
			nftID = output.Address().(*devnetvm2.AliasAddress)
		}
	}

	wallet.markOutputsAndAddressesSpent(consumedOutputs)

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, nil, err
	}
	if createNFTOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}

	return tx, nftID, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransferNFT //////////////////////////////////////////////////////////////////////////////////////////////////

// TransferNFT transfers an NFT to a given address.
func (wallet *Wallet) TransferNFT(options ...transfernftoptions.TransferNFTOption) (tx *devnetvm2.Transaction, err error) {
	transferOptions, err := transfernftoptions.Build(options...)
	if err != nil {
		return
	}

	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(transferOptions.AccessManaPledgeID, transferOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	// look up if we have the alias output
	walletAlias, err := wallet.findGovernedAliasOutputByAliasID(transferOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*devnetvm2.AliasOutput)
	if alias.DelegationTimeLockedNow(time.Now()) {
		err = errors.Errorf("alias %s is delegation timelocked until %s", alias.GetAliasAddress().Base58(),
			alias.DelegationTimelock().String())
		return
	}

	// check if we are not trying to governance deadlock
	// Note, that a deadlock means that aliases circularly govern each other. Such aliases will not be able to get
	// governance unlocked due to protocol constraints.
	//  - an alias cannot govern itself, so level 1 circular dependency is covered by the syntactic checks of outputs
	//  - here we can check if the other alias is governed by us, hence preventing level 2 circular dependency
	//  - but we can't prevent level 3 or greater circular dependency without walking the governing path, which can be
	//    expensive.
	if transferOptions.ToAddress.Type() == devnetvm2.AliasAddressType {
		// we are giving the governor role to another alias. Is that other alias governed by this alias?
		var otherAlias *devnetvm2.AliasOutput
		otherAlias, err = wallet.connector.GetUnspentAliasOutput(transferOptions.ToAddress.(*devnetvm2.AliasAddress))
		if err != nil {
			err = errors.Errorf("failed to check that transfer wouldn't result in deadlocked outputs: %w", err)
			return
		}
		if otherAlias.GetGoverningAddress().Equals(alias.GetAliasAddress()) {
			err = errors.Errorf("transfer of nft to %s would result in circular alias governance", transferOptions.ToAddress.Base58())
			return
		}
	}

	// transfer means we are transferring the governor role, so it has to be a governance update
	nextAlias := alias.NewAliasOutputNext(true)
	if nextAlias.IsSelfGoverned() {
		err = nextAlias.SetStateAddress(transferOptions.ToAddress)
		if err != nil {
			return
		}
	} else {
		if transferOptions.ResetStateAddress {
			// we make it self governed for the receive address
			nextAlias.SetGoverningAddress(nil)
			err = nextAlias.SetStateAddress(transferOptions.ToAddress)
			if err != nil {
				return
			}
		} else {
			// only transfer the governor role, state controller remains.
			nextAlias.SetGoverningAddress(transferOptions.ToAddress)
		}
	}

	if transferOptions.ResetDelegation {
		nextAlias.SetIsDelegated(false)
	}

	essence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID,
		devnetvm2.NewInputs(alias.Input()),
		devnetvm2.NewOutputs(nextAlias),
	)
	// there is only one input, so signing is easy
	keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
	tx = devnetvm2.NewTransaction(essence, devnetvm2.UnlockBlocks{
		devnetvm2.NewSignatureUnlockBlock(devnetvm2.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(essence.Bytes())))),
	})

	// check syntactical validity by marshaling an unmarshaling
	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	err = new(devnetvm2.Transaction).FromBytes(txBytes)
	if err != nil {
		return nil, err
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(devnetvm2.Outputs{alias}, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(OutputsByAddressAndOutputID{walletAlias.Address: {
		walletAlias.Object.ID(): walletAlias,
	}})

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if transferOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}

	return tx, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DestroyNFT ///////////////////////////////////////////////////////////////////////////////////////////////////

// DestroyNFT destroys the given nft (alias).
func (wallet *Wallet) DestroyNFT(options ...destroynftoptions.DestroyNFTOption) (tx *devnetvm2.Transaction, err error) {
	destroyOptions, err := destroynftoptions.Build(options...)
	if err != nil {
		return
	}
	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(destroyOptions.AccessManaPledgeID, destroyOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}
	// look up if we have the alias output
	walletAlias, err := wallet.findGovernedAliasOutputByAliasID(destroyOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*devnetvm2.AliasOutput)

	if alias.DelegationTimeLockedNow(time.Now()) {
		err = errors.Errorf("alias %s is delegation timelocked until %s", alias.GetAliasAddress().Base58(), alias.DelegationTimelock().String())
		return
	}

	// can only be destroyed when minimal funds are present (unless it is delegated)
	if !alias.IsDelegated() && !devnetvm2.IsExactDustMinimum(alias.Balances()) {
		withdrawAmount := alias.Balances().Map()
		withdrawAmount[devnetvm2.ColorIOTA] -= devnetvm2.DustThresholdAliasOutputIOTA
		_, err = wallet.WithdrawFundsFromNFT(
			withdrawfromnftoptions.Alias(destroyOptions.Alias.Base58()),
			withdrawfromnftoptions.Amount(withdrawAmount),
			withdrawfromnftoptions.WaitForConfirmation(true),
		)
		if err != nil {
			return
		}
		walletAlias, err = wallet.findGovernedAliasOutputByAliasID(destroyOptions.Alias)
		if err != nil {
			return
		}
		alias = walletAlias.Object.(*devnetvm2.AliasOutput)
	}

	// determine where the remainder will go
	consumedOutputs := OutputsByAddressAndOutputID{
		// we only consume the to-be-destroyed alias
		walletAlias.Address: {walletAlias.Object.ID(): walletAlias},
	}
	remainderAddy := wallet.chooseRemainderAddress(consumedOutputs, address.AddressEmpty)
	remainderOutput := devnetvm2.NewSigLockedColoredOutput(alias.Balances(), remainderAddy.Address())

	inputs := devnetvm2.Inputs{alias.Input()}
	outputs := devnetvm2.Outputs{remainderOutput}
	essence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID,
		devnetvm2.NewInputs(inputs...), devnetvm2.NewOutputs(outputs...))

	// there is only one input, so signing is easy
	keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
	tx = devnetvm2.NewTransaction(essence, devnetvm2.UnlockBlocks{
		devnetvm2.NewSignatureUnlockBlock(devnetvm2.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(essence.Bytes())))),
	})

	// check syntactical validity by marshaling an unmarshaling
	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	err = new(devnetvm2.Transaction).FromBytes(txBytes)
	if err != nil {
		return nil, err
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(devnetvm2.Outputs{alias}, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(OutputsByAddressAndOutputID{walletAlias.Address: {
		walletAlias.Object.ID(): walletAlias,
	}})

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if destroyOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}

	return tx, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithdrawFundsFromNFT /////////////////////////////////////////////////////////////////////////////////////////

// WithdrawFundsFromNFT withdraws funds from the given alias. If the wallet is not the state controller, or too much funds
// are withdrawn, an error is returned.
func (wallet *Wallet) WithdrawFundsFromNFT(options ...withdrawfromnftoptions.WithdrawFundsFromNFTOption) (tx *devnetvm2.Transaction, err error) {
	withdrawOptions, err := withdrawfromnftoptions.Build(options...)
	if err != nil {
		return
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(withdrawOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*devnetvm2.AliasOutput)
	balancesOfAlias := alias.Balances()
	withdrawBalances := withdrawOptions.Amount
	newAliasBalance := map[devnetvm2.Color]uint64{}

	// check if withdrawBalance is valid for alias
	balancesOfAlias.ForEach(func(color devnetvm2.Color, balance uint64) bool {
		if balance < withdrawBalances[color] {
			err = errors.Errorf("trying to withdraw %d %s tokens from alias, but there are only %d tokens in it",
				withdrawBalances[color], color.Base58(), balance)
			return false
		}
		newAliasBalance[color] = balance - withdrawBalances[color]
		if newAliasBalance[color] == 0 {
			delete(newAliasBalance, color)
		}
		if color == devnetvm2.ColorIOTA && newAliasBalance[color] < devnetvm2.DustThresholdAliasOutputIOTA {
			err = errors.Errorf("%d IOTA tokens would remain after withdrawal, which is less, then the minimum required %d",
				newAliasBalance[color], devnetvm2.DustThresholdAliasOutputIOTA)
			return false
		}
		return true
	})
	if err != nil {
		return
	}

	nextAlias := alias.NewAliasOutputNext(false)
	err = nextAlias.SetBalances(newAliasBalance)
	if err != nil {
		return
	}

	consumedOutputs := OutputsByAddressAndOutputID{
		// we only consume the to-be-destroyed alias
		walletAlias.Address: {walletAlias.Object.ID(): walletAlias},
	}
	var optionsToAddress address.Address
	if withdrawOptions.ToAddress == nil {
		optionsToAddress = address.AddressEmpty
	} else {
		optionsToAddress = address.Address{AddressBytes: withdrawOptions.ToAddress.Array()}
	}
	remainderAddress := wallet.chooseRemainderAddress(consumedOutputs, optionsToAddress)

	remainderOutput := devnetvm2.NewSigLockedColoredOutput(devnetvm2.NewColoredBalances(withdrawBalances), remainderAddress.Address())

	inputs := devnetvm2.Inputs{alias.Input()}
	outputs := devnetvm2.Outputs{remainderOutput, nextAlias}

	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(withdrawOptions.AccessManaPledgeID, withdrawOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	essence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID,
		devnetvm2.NewInputs(inputs...), devnetvm2.NewOutputs(outputs...))

	// there is only one input, so signing is easy
	keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
	tx = devnetvm2.NewTransaction(essence, devnetvm2.UnlockBlocks{
		devnetvm2.NewSignatureUnlockBlock(devnetvm2.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(essence.Bytes())))),
	})

	// check syntactical validity by marshaling an unmarshaling
	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	err = new(devnetvm2.Transaction).FromBytes(txBytes)
	if err != nil {
		return nil, err
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(devnetvm2.Outputs{alias}, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(OutputsByAddressAndOutputID{walletAlias.Address: {
		walletAlias.Object.ID(): walletAlias,
	}})

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if withdrawOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}

	return tx, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DepositFundsToNFT ////////////////////////////////////////////////////////////////////////////////////////////

// DepositFundsToNFT deposits funds to the given alias from the wallet funds. If the wallet is not the state controller, an error is returned.
func (wallet *Wallet) DepositFundsToNFT(options ...deposittonftoptions.DepositFundsToNFTOption) (tx *devnetvm2.Transaction, err error) {
	depositOptions, err := deposittonftoptions.Build(options...)
	if err != nil {
		return
	}
	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(depositOptions.AccessManaPledgeID, depositOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(depositOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*devnetvm2.AliasOutput)
	depositBalances := depositOptions.Amount
	newAliasBalance := alias.Balances().Map() // we are going to top it up with depositbalances
	// add deposit balances to alias balance
	for color, balance := range depositBalances {
		newAliasBalance[color] += balance
	}

	// collect funds required for a deposit
	consumedOutputs, err := wallet.collectOutputsForFunding(depositBalances, false)
	if err != nil {
		if errors.Is(err, ErrTooManyOutputs) {
			err = errors.Errorf("consolidate funds and try again: %w", err)
		}
		return nil, err
	}
	// build inputs from consumed outputs
	inputsFromConsumedOutputs := wallet.buildInputs(consumedOutputs)
	// add the alias
	unsortedInputs := append(inputsFromConsumedOutputs, alias.Input())
	// sort all inputs
	inputs := devnetvm2.NewInputs(unsortedInputs...)
	// aggregate all the funds we consume from inputs used to fund the deposit (there is the alias input as well)
	totalConsumed := consumedOutputs.TotalFundsInOutputs()
	// create the alias state transition (only state transition can modify balance)
	nextAlias := alias.NewAliasOutputNext(false)
	// update the balance of the deposited nft output
	err = nextAlias.SetBalances(newAliasBalance)
	if err != nil {
		return nil, err
	}
	unsortedOutputs := devnetvm2.Outputs{nextAlias}

	// remainder balance = totalConsumed - deposit
	for color, balance := range depositBalances {
		if totalConsumed[color] < balance {
			return nil, errors.Errorf("deposit funds are greater than consumed funds")
		}
		totalConsumed[color] -= balance
		if totalConsumed[color] <= 0 {
			delete(totalConsumed, color)
		}
	}
	remainderBalances := devnetvm2.NewColoredBalances(totalConsumed)
	// only add remainder output if there is a remainder balance
	if remainderBalances.Size() != 0 {
		unsortedOutputs = append(unsortedOutputs, devnetvm2.NewSigLockedColoredOutput(
			remainderBalances, wallet.chooseRemainderAddress(consumedOutputs, address.AddressEmpty).Address()))
	}

	// create tx essence
	outputs := devnetvm2.NewOutputs(unsortedOutputs...)
	txEssence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, inputs, outputs)
	// add the alias to the consumed outputs
	if _, exists := consumedOutputs[walletAlias.Address]; !exists {
		consumedOutputs[walletAlias.Address] = make(map[utxo.OutputID]*Output)
	}
	consumedOutputs[walletAlias.Address][walletAlias.Object.ID()] = walletAlias

	// build unlock blocks
	unlockBlocks, inputsInOrder := wallet.buildUnlockBlocks(inputs, consumedOutputs.OutputsByID(), txEssence)

	tx = devnetvm2.NewTransaction(txEssence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	err = new(devnetvm2.Transaction).FromBytes(txBytes)
	if err != nil {
		return nil, err
	}
	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(consumedOutputs)

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if depositOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}

	return tx, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SweepNFTOwnedFunds ///////////////////////////////////////////////////////////////////////////////////////////

// SweepNFTOwnedFunds collects all funds from non-alias outputs that are owned by the nft into the wallet.
func (wallet Wallet) SweepNFTOwnedFunds(options ...sweepnftownedoptions.SweepNFTOwnedFundsOption) (tx *devnetvm2.Transaction, err error) {
	sweepOptions, err := sweepnftownedoptions.Build(options...)
	if err != nil {
		return
	}
	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(sweepOptions.AccessManaPledgeID, sweepOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}
	// do we own the nft as a state controller?
	_, stateControlled, _, _, err := wallet.AliasBalance()
	if err != nil {
		return
	}
	if _, has := stateControlled[*sweepOptions.Alias]; !has {
		err = errors.Errorf("nft %s is not state controlled by the wallet", sweepOptions.Alias.Base58())
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(sweepOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*devnetvm2.AliasOutput)

	owned, _, err := wallet.AvailableOutputsOnNFT(sweepOptions.Alias.Base58())
	if err != nil {
		return
	}
	if len(owned) == 0 {
		err = errors.Errorf("no owned outputs with funds are found on nft %s", sweepOptions.Alias.Base58())
		return
	}

	toBeConsumed := devnetvm2.Outputs{}
	totalConsumed := map[devnetvm2.Color]uint64{}
	// owned contains all outputs that are owned by nft. we want to filter out alias outputs, as they are not "funds"
	for _, output := range owned {
		if len(toBeConsumed) == devnetvm2.MaxOutputCount-1 {
			// we can spend at most 127 inputs in a tx, need one more for the alias
			break
		}
		if output.Type() == devnetvm2.AliasOutputType {
			continue
		}
		output.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
			totalConsumed[color] += balance
			return true
		})
		toBeConsumed = append(toBeConsumed, output)
	}
	if len(toBeConsumed) == 0 {
		err = errors.Errorf("no owned outputs with funds are found on nft %s", sweepOptions.Alias.Base58())
		return
	}

	nextAlias := alias.NewAliasOutputNext(false)
	toBeConsumed = append(toBeConsumed, alias)

	var optionsToAddress address.Address
	if sweepOptions.ToAddress == nil {
		optionsToAddress = address.AddressEmpty
	} else {
		optionsToAddress = address.Address{AddressBytes: sweepOptions.ToAddress.Array()}
	}
	consumedOutputs := OutputsByAddressAndOutputID{
		// we only consume the to-be-destroyed alias from the wallet
		walletAlias.Address: {walletAlias.Object.ID(): walletAlias},
	}
	toAddress := wallet.chooseToAddress(consumedOutputs, optionsToAddress)

	unsortedInputs := toBeConsumed.Inputs()
	unsortedOutputs := devnetvm2.Outputs{nextAlias, devnetvm2.NewSigLockedColoredOutput(devnetvm2.NewColoredBalances(totalConsumed), toAddress.Address())}

	essence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, devnetvm2.NewInputs(unsortedInputs...), devnetvm2.NewOutputs(unsortedOutputs...))

	toBeConsumeByID := toBeConsumed.ByID()
	inputsInOrder := devnetvm2.Outputs{}
	unlockBlocks := make(devnetvm2.UnlockBlocks, len(essence.Inputs()))
	aliasInputIndex := -1
	// find the input of alias
	for index, input := range essence.Inputs() {
		if input.Type() == devnetvm2.UTXOInputType {
			casted := input.(*devnetvm2.UTXOInput)
			if casted.ReferencedOutputID() == alias.ID() {
				keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
				unlockBlock := devnetvm2.NewSignatureUnlockBlock(devnetvm2.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(essence.Bytes()))))
				unlockBlocks[index] = unlockBlock
				aliasInputIndex = index
			}
			inputsInOrder = append(inputsInOrder, toBeConsumeByID[casted.ReferencedOutputID()])
		}
	}
	if aliasInputIndex < 0 {
		err = errors.Errorf("failed to find alias %s among prepared transaction inputs", alias.GetAliasAddress().Base58())
		return
	}
	// fill rest of the unlock blocks
	for i := range essence.Inputs() {
		if i != aliasInputIndex {
			unlockBlocks[i] = devnetvm2.NewAliasUnlockBlock(uint16(aliasInputIndex))
		}
	}

	tx = devnetvm2.NewTransaction(essence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	txBytes, err := tx.Bytes()
	if err != nil {
		return nil, err
	}
	err = new(devnetvm2.Transaction).FromBytes(txBytes)
	if err != nil {
		return nil, err
	}

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Errorf("created transaction is invalid: %s", tx.String())
	}

	wallet.markOutputsAndAddressesSpent(OutputsByAddressAndOutputID{walletAlias.Address: {
		walletAlias.Object.ID(): walletAlias,
	}})

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if sweepOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}

	return tx, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SweepNFTOwnedNFTs ////////////////////////////////////////////////////////////////////////////////////////////

func (wallet *Wallet) SweepNFTOwnedNFTs(options ...sweepnftownednftsoptions.SweepNFTOwnedNFTsOption) (tx *devnetvm2.Transaction, sweptNFTs []*devnetvm2.AliasAddress, err error) {
	sweepOptions, err := sweepnftownednftsoptions.Build(options...)
	if err != nil {
		return
	}
	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(sweepOptions.AccessManaPledgeID, sweepOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	// do we own the nft as a state controller?
	_, stateControlled, _, _, err := wallet.AliasBalance()
	if err != nil {
		return
	}
	if _, has := stateControlled[*sweepOptions.Alias]; !has {
		err = errors.Errorf("nft %s is not state controlled by the wallet", sweepOptions.Alias.Base58())
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(sweepOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*devnetvm2.AliasOutput)
	owned, _, err := wallet.AvailableOutputsOnNFT(sweepOptions.Alias.Base58())
	if err != nil {
		return
	}
	if len(owned) == 0 {
		err = errors.Errorf("no owned outputs with funds are found on nft %s", sweepOptions.Alias.Base58())
	}
	toBeConsumed := devnetvm2.Outputs{}
	// owned contains all outputs that are owned by nft. we want to filter out non alias outputs
	now := time.Now()
	for _, output := range owned {
		if len(toBeConsumed) == devnetvm2.MaxInputCount-1 {
			// we can spend at most 127 inputs in a tx, need one more for the alias
			break
		}
		if output.Type() == devnetvm2.AliasOutputType {
			casted := output.(*devnetvm2.AliasOutput)
			if casted.DelegationTimeLockedNow(now) {
				// the output is delegation timelocked at the moment, so the governor can't move it
				continue
			}
			toBeConsumed = append(toBeConsumed, output)
		}
	}
	// determine which address to send to
	var optionsToAddress address.Address
	if sweepOptions.ToAddress == nil {
		optionsToAddress = address.AddressEmpty
	} else {
		optionsToAddress = address.Address{AddressBytes: sweepOptions.ToAddress.Array()}
	}
	consumedOutputs := OutputsByAddressAndOutputID{
		// we only consume the to-be-destroyed alias from the wallet
		walletAlias.Address: {walletAlias.Object.ID(): walletAlias},
	}
	toAddress := wallet.chooseToAddress(consumedOutputs, optionsToAddress)
	// nextAlias is the nft we control
	nextAlias := alias.NewAliasOutputNext(false)
	// transition nft owned aliases
	unsortedOutputs := devnetvm2.Outputs{nextAlias}
	for _, output := range toBeConsumed {
		if output.Type() != devnetvm2.AliasOutputType {
			continue
		}
		next := output.(*devnetvm2.AliasOutput).NewAliasOutputNext(true)
		// set to self-governed by toAddress
		next.SetGoverningAddress(nil)
		err = next.SetStateAddress(toAddress.Address())
		if err != nil {
			return
		}
		unsortedOutputs = append(unsortedOutputs, next)
	}
	// we will consume the nft that owns the others too
	toBeConsumed = append(toBeConsumed, alias)
	unsortedInputs := toBeConsumed.Inputs()

	// create essence, contains sorted inputs and outputs
	essence := devnetvm2.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, devnetvm2.NewInputs(unsortedInputs...), devnetvm2.NewOutputs(unsortedOutputs...))

	toBeConsumeByID := toBeConsumed.ByID()
	inputsInOrder := devnetvm2.Outputs{}
	unlockBlocks := make(devnetvm2.UnlockBlocks, len(essence.Inputs()))
	aliasInputIndex := -1
	// find the input of alias
	for index, input := range essence.Inputs() {
		if input.Type() == devnetvm2.UTXOInputType {
			casted := input.(*devnetvm2.UTXOInput)
			if casted.ReferencedOutputID() == alias.ID() {
				keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
				unlockBlock := devnetvm2.NewSignatureUnlockBlock(devnetvm2.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(essence.Bytes()))))
				unlockBlocks[index] = unlockBlock
				aliasInputIndex = index
			}
			inputsInOrder = append(inputsInOrder, toBeConsumeByID[casted.ReferencedOutputID()])
		}
	}
	if aliasInputIndex < 0 {
		err = errors.Errorf("failed to find alias %s among prepared transaction inputs", alias.GetAliasAddress().Base58())
		return
	}
	// fill rest of the unlock blocks
	for i := range essence.Inputs() {
		if i != aliasInputIndex {
			unlockBlocks[i] = devnetvm2.NewAliasUnlockBlock(uint16(aliasInputIndex))
		}
	}

	tx = devnetvm2.NewTransaction(essence, unlockBlocks)

	// check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return
	}
	if !ok {
		err = errors.Errorf("created transaction is invalid: %s", tx.String())
		return
	}

	wallet.markOutputsAndAddressesSpent(OutputsByAddressAndOutputID{walletAlias.Address: {
		walletAlias.Object.ID(): walletAlias,
	}})

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return
	}

	for _, output := range tx.Essence().Outputs() {
		if output.Type() == devnetvm2.AliasOutputType {
			casted := output.(*devnetvm2.AliasOutput)
			if casted.GetAliasAddress().Equals(alias.GetAliasAddress()) {
				// we skip the owned nft
				continue
			}
			sweptNFTs = append(sweptNFTs, casted.GetAliasAddress())
		}
	}

	if sweepOptions.WaitForConfirmation {
		err = wallet.WaitForTxAcceptance(tx.ID())
	}
	return tx, sweptNFTs, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ServerStatus /////////////////////////////////////////////////////////////////////////////////////////////////

// ServerStatus retrieves the connected server status.
func (wallet *Wallet) ServerStatus() (status ServerStatus, err error) {
	return wallet.connector.(*WebConnector).ServerStatus()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AllowedPledgeNodeIDs /////////////////////////////////////////////////////////////////////////////////////////

// AllowedPledgeNodeIDs retrieves the allowed pledge node IDs.
func (wallet *Wallet) AllowedPledgeNodeIDs() (res map[mana2.Type][]string, err error) {
	return wallet.connector.(*WebConnector).GetAllowedPledgeIDs()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AssetRegistry ////////////////////////////////////////////////////////////////////////////////////////////////

// AssetRegistry return the internal AssetRegistry instance of the wallet.
func (wallet *Wallet) AssetRegistry() *AssetRegistry {
	return wallet.assetRegistry
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReceiveAddress ///////////////////////////////////////////////////////////////////////////////////////////////

// ReceiveAddress returns the last receive address of the wallet.
func (wallet *Wallet) ReceiveAddress() address.Address {
	return wallet.addressManager.LastUnspentAddress()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region NewReceiveAddress ////////////////////////////////////////////////////////////////////////////////////////////

// NewReceiveAddress generates and returns a new unused receive address.
func (wallet *Wallet) NewReceiveAddress() address.Address {
	return wallet.addressManager.NewAddress()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RemainderAddress /////////////////////////////////////////////////////////////////////////////////////////////

// RemainderAddress returns the address that is used for the remainder of funds.
func (wallet *Wallet) RemainderAddress() address.Address {
	return wallet.addressManager.FirstUnspentAddress()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnspentOutputs ///////////////////////////////////////////////////////////////////////////////////////////////

// UnspentOutputs returns the unspent outputs that are available for spending.
func (wallet *Wallet) UnspentOutputs() map[address.Address]map[utxo.OutputID]*Output {
	return wallet.outputManager.UnspentOutputs(false)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnspentValueOutputs //////////////////////////////////////////////////////////////////////////////////////////

// UnspentValueOutputs returns the unspent value type outputs that are available for spending.
func (wallet *Wallet) UnspentValueOutputs() map[address.Address]map[utxo.OutputID]*Output {
	return wallet.outputManager.UnspentValueOutputs(false)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnspentAliasOutputs //////////////////////////////////////////////////////////////////////////////////////////

// UnspentAliasOutputs returns the unspent alias outputs that are available for spending.
func (wallet *Wallet) UnspentAliasOutputs(includePending bool) map[address.Address]map[utxo.OutputID]*Output {
	return wallet.outputManager.UnspentAliasOutputs(includePending)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RequestFaucetFunds ///////////////////////////////////////////////////////////////////////////////////////////

// RequestFaucetFunds requests some funds from the faucet for testing purposes.
func (wallet *Wallet) RequestFaucetFunds(waitForConfirmation ...bool) (err error) {
	if len(waitForConfirmation) == 0 || !waitForConfirmation[0] {
		err = wallet.connector.RequestFaucetFunds(wallet.ReceiveAddress(), wallet.faucetPowDifficulty)

		return
	}

	if err = wallet.Refresh(); err != nil {
		return
	}
	confirmedBalance, _, err := wallet.Balance()
	if err != nil {
		return
	}

	err = wallet.connector.RequestFaucetFunds(wallet.ReceiveAddress(), wallet.faucetPowDifficulty)
	if err != nil {
		return
	}
	err = wallet.waitForBalanceConfirmation(confirmedBalance)
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Refresh //////////////////////////////////////////////////////////////////////////////////////////////////////

// Refresh scans the addresses for incoming transactions. If the optional rescanSpentAddresses parameter is set to true
// we also scan the spent addresses again (this can take longer).
func (wallet *Wallet) Refresh(rescanSpentAddresses ...bool) (err error) {
	err = wallet.outputManager.Refresh(rescanSpentAddresses...)
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Balance //////////////////////////////////////////////////////////////////////////////////////////////////////

// Balance returns the confirmed and pending balance of the funds managed by this wallet.
func (wallet *Wallet) Balance(refresh ...bool) (confirmedBalance, pendingBalance map[devnetvm2.Color]uint64, err error) {
	shouldRefresh := true
	if len(refresh) > 0 {
		shouldRefresh = refresh[0]
	}
	if shouldRefresh {
		err = wallet.outputManager.Refresh()
		if err != nil {
			return
		}
	}

	confirmedBalance = make(map[devnetvm2.Color]uint64)
	pendingBalance = make(map[devnetvm2.Color]uint64)

	// iterate through the unspent outputs
	for addy, outputsOnAddress := range wallet.outputManager.UnspentOutputs(true) {
		for _, output := range outputsOnAddress {
			// determine target map
			var targetMap map[devnetvm2.Color]uint64
			if output.ConfirmationStateReached {
				targetMap = confirmedBalance
			} else {
				targetMap = pendingBalance
			}

			switch output.Object.Type() {
			case devnetvm2.SigLockedSingleOutputType:
			case devnetvm2.SigLockedColoredOutputType:
				// extract balance
				output.Object.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
					targetMap[color] += balance
					return true
				})
			case devnetvm2.ExtendedLockedOutputType:
				casted := output.Object.(*devnetvm2.ExtendedLockedOutput)
				unlockAddyNow := casted.UnlockAddressNow(time.Now())
				if addy.Address().Equals(unlockAddyNow) {
					// we own this output now
					casted.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
						targetMap[color] += balance
						return true
					})
				}
			case devnetvm2.AliasOutputType:
				casted := output.Object.(*devnetvm2.AliasOutput)
				if casted.IsDelegated() {
					continue
				}
				if casted.IsSelfGoverned() {
					// if it is self governed, addy is the state address, so we own everything
					casted.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
						targetMap[color] += balance
						return true
					})
					continue
				}
				if casted.GetStateAddress().Equals(addy.Address()) {
					// we are state controller
					casted.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
						if color == devnetvm2.ColorIOTA {
							// the minimum amount can only be moved by the governor
							surplusIOTA := balance - devnetvm2.DustThresholdAliasOutputIOTA
							if surplusIOTA == 0 {
								return true
							}
							targetMap[color] += surplusIOTA
						} else {
							targetMap[color] += balance
						}
						return true
					})
					continue
				}
				if casted.GetGoverningAddress().Equals(addy.Address()) {
					// we are the governor, so we only own the minimum dust amount that cannot be withdrawn by the state controller
					targetMap[devnetvm2.ColorIOTA] += devnetvm2.DustThresholdAliasOutputIOTA
					continue
				}
			}
		}
	}

	return confirmedBalance, pendingBalance, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AvailableBalance /////////////////////////////////////////////////////////////////////////////////////////////

// AvailableBalance returns the balance that is not held in aliases, and therefore can be used to fund transfers.
func (wallet *Wallet) AvailableBalance(refresh ...bool) (confirmedBalance, pendingBalance map[devnetvm2.Color]uint64, err error) {
	shouldRefresh := true
	if len(refresh) > 0 {
		shouldRefresh = refresh[0]
	}
	if shouldRefresh {
		err = wallet.outputManager.Refresh()
		if err != nil {
			return
		}
	}

	confirmedBalance = make(map[devnetvm2.Color]uint64)
	pendingBalance = make(map[devnetvm2.Color]uint64)
	now := time.Now()
	// iterate through the unspent outputs
	for addy, outputsOnAddress := range wallet.outputManager.UnspentOutputs(true) {
		for _, output := range outputsOnAddress {
			// determine target map
			var targetMap map[devnetvm2.Color]uint64
			if output.ConfirmationStateReached {
				targetMap = confirmedBalance
			} else {
				targetMap = pendingBalance
			}

			switch output.Object.Type() {
			case devnetvm2.SigLockedSingleOutputType:
			case devnetvm2.SigLockedColoredOutputType:
				// extract balance
				output.Object.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
					targetMap[color] += balance
					return true
				})
			case devnetvm2.ExtendedLockedOutputType:
				casted := output.Object.(*devnetvm2.ExtendedLockedOutput)
				if casted.TimeLockedNow(now) {
					// timelocked funds are not available
					continue
				}
				unlockAddyNow := casted.UnlockAddressNow(now)
				if addy.Address().Equals(unlockAddyNow) {
					// we own this output now
					casted.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
						targetMap[color] += balance
						return true
					})
				}
			}
		}
	}

	return confirmedBalance, pendingBalance, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimelockedBalances ///////////////////////////////////////////////////////////////////////////////////////////

// TimelockedBalances returns all confirmed and pending balances that are currently timelocked.
func (wallet *Wallet) TimelockedBalances(refresh ...bool) (confirmed, pending TimedBalanceSlice, err error) {
	shouldRefresh := true
	if len(refresh) > 0 {
		shouldRefresh = refresh[0]
	}
	if shouldRefresh {
		err = wallet.outputManager.Refresh()
		if err != nil {
			return
		}
	}

	confirmed = make([]*TimedBalance, 0)
	pending = make([]*TimedBalance, 0)
	now := time.Now()

	// iterate through the unspent outputs
	for _, outputsOnAddress := range wallet.outputManager.UnspentOutputs(true) {
		for _, output := range outputsOnAddress {
			if output.Object.Type() != devnetvm2.ExtendedLockedOutputType {
				continue
			}
			casted := output.Object.(*devnetvm2.ExtendedLockedOutput)
			if casted.TimeLockedNow(now) {
				tBal := &TimedBalance{
					Balance: casted.Balances().Map(),
					Time:    casted.TimeLock(),
				}
				if output.ConfirmationStateReached {
					confirmed = append(confirmed, tBal)
				} else {
					pending = append(pending, tBal)
				}
			}
		}
	}

	return confirmed, pending, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConditionalBalances //////////////////////////////////////////////////////////////////////////////////////////

// ConditionalBalances returns all confirmed and pending balances that can be claimed by the wallet up to a certain time.
func (wallet *Wallet) ConditionalBalances(refresh ...bool) (confirmed, pending TimedBalanceSlice, err error) {
	shouldRefresh := true
	if len(refresh) > 0 {
		shouldRefresh = refresh[0]
	}
	if shouldRefresh {
		err = wallet.outputManager.Refresh()
		if err != nil {
			return
		}
	}

	confirmed = make(TimedBalanceSlice, 0)
	pending = make(TimedBalanceSlice, 0)
	now := time.Now()

	// iterate through the unspent outputs
	for addy, outputsOnAddress := range wallet.outputManager.UnspentOutputs(true) {
		for _, output := range outputsOnAddress {
			if output.Object.Type() != devnetvm2.ExtendedLockedOutputType {
				continue
			}
			casted := output.Object.(*devnetvm2.ExtendedLockedOutput)
			_, fallbackDeadline := casted.FallbackOptions()
			if !fallbackDeadline.IsZero() && addy.Address().Equals(casted.UnlockAddressNow(now)) {
				// fallback option is set and currently we are the unlock address
				cBal := &TimedBalance{
					Balance: casted.Balances().Map(),
					Time:    fallbackDeadline,
				}
				if output.ConfirmationStateReached {
					confirmed = append(confirmed, cBal)
				} else {
					pending = append(pending, cBal)
				}
			}
		}
	}

	return confirmed, pending, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasBalance /////////////////////////////////////////////////////////////////////////////////////////////////

// AliasBalance returns the aliases held by this wallet.
func (wallet *Wallet) AliasBalance(refresh ...bool) (
	confirmedGovernedAliases,
	confirmedStateControlledAliases,
	pendingGovernedAliases,
	pendingStateControlledAliases map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput,
	err error,
) {
	confirmedGovernedAliases = map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput{}
	confirmedStateControlledAliases = map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput{}
	pendingGovernedAliases = map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput{}
	pendingStateControlledAliases = map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput{}
	shouldRefresh := true
	if len(refresh) > 0 {
		shouldRefresh = refresh[0]
	}
	if shouldRefresh {
		err = wallet.outputManager.Refresh()
		if err != nil {
			return
		}
	}

	aliasOutputs := wallet.UnspentAliasOutputs(true)

	for addr, outputIDToOutputMap := range aliasOutputs {
		for _, output := range outputIDToOutputMap {
			if output.Object.Type() != devnetvm2.AliasOutputType {
				continue
			}
			// target maps
			var governedAliases, stateControlledAliases map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput
			if output.ConfirmationStateReached {
				governedAliases = confirmedGovernedAliases
				stateControlledAliases = confirmedStateControlledAliases
			} else {
				governedAliases = pendingGovernedAliases
				stateControlledAliases = pendingStateControlledAliases
			}
			alias := output.Object.(*devnetvm2.AliasOutput)
			if alias.GetGoverningAddress().Equals(addr.Address()) {
				// alias is governed by the wallet
				governedAliases[*alias.GetAliasAddress()] = alias
			}
			if alias.GetStateAddress().Equals(addr.Address()) {
				// alias is state controlled by the wallet
				stateControlledAliases[*alias.GetAliasAddress()] = alias
			}
		}
	}
	return confirmedGovernedAliases, confirmedStateControlledAliases, pendingGovernedAliases, pendingStateControlledAliases, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AvailableOutputsOnNFT ////////////////////////////////////////////////////////////////////////////////////////

// AvailableOutputsOnNFT returns all outputs that are either owned (SigLocked***, Extended, stateControlled Alias) or governed
// (governance controlled alias outputs) and are not currently locked.
func (wallet Wallet) AvailableOutputsOnNFT(nftID string) (owned, governed devnetvm2.Outputs, err error) {
	aliasAddress, err := devnetvm2.AliasAddressFromBase58EncodedString(nftID)
	if err != nil {
		return
	}
	res, err := wallet.connector.UnspentOutputs(address.Address{AddressBytes: aliasAddress.Array()})
	if err != nil {
		return
	}
	outputs := res.ToLedgerStateOutputs()
	now := time.Now()
	for _, o := range outputs {
		switch o.Type() {
		case devnetvm2.SigLockedSingleOutputType, devnetvm2.SigLockedColoredOutputType:
			owned = append(owned, o)
		case devnetvm2.ExtendedLockedOutputType:
			casted := o.(*devnetvm2.ExtendedLockedOutput)
			if casted.UnlockAddressNow(now).Equals(aliasAddress) && !casted.TimeLockedNow(now) {
				owned = append(owned, o)
			}
		case devnetvm2.AliasOutputType:
			casted := o.(*devnetvm2.AliasOutput)
			// the alias output of aliasAddress is filtered out
			if casted.GetStateAddress().Equals(aliasAddress) && !casted.DelegationTimeLockedNow(now) {
				owned = append(owned, o)
			} else if casted.GetGoverningAddress().Equals(aliasAddress) && !casted.DelegationTimeLockedNow(now) {
				governed = append(governed, o)
			}
		}
	}
	return owned, governed, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DelegatedAliasBalance ////////////////////////////////////////////////////////////////////////////////////////

// DelegatedAliasBalance returns the pending and confirmed aliases that are delegated.
func (wallet *Wallet) DelegatedAliasBalance(refresh ...bool) (
	confirmedDelegatedAliases map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput,
	pendingDelegatedAliases map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput,
	err error,
) {
	confirmedDelegatedAliases = map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput{}
	pendingDelegatedAliases = map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput{}

	shouldRefresh := true
	if len(refresh) > 0 {
		shouldRefresh = refresh[0]
	}
	if shouldRefresh {
		err = wallet.outputManager.Refresh()
		if err != nil {
			return
		}
	}

	aliasOutputs := wallet.UnspentAliasOutputs(true)

	for addr, outputIDToOutputMap := range aliasOutputs {
		for _, output := range outputIDToOutputMap {
			if output.Object.Type() != devnetvm2.AliasOutputType {
				continue
			}
			alias := output.Object.(*devnetvm2.AliasOutput)
			// skip if the output was delegated
			if !alias.IsDelegated() {
				continue
			}
			// target maps
			var delegatedAliases map[devnetvm2.AliasAddress]*devnetvm2.AliasOutput
			if output.ConfirmationStateReached {
				delegatedAliases = confirmedDelegatedAliases
			} else {
				delegatedAliases = pendingDelegatedAliases
			}
			if alias.GetGoverningAddress().Equals(addr.Address()) {
				// alias is governed by the wallet (and we previously checked that it is delegated)
				delegatedAliases[*alias.GetAliasAddress()] = alias
			}
		}
	}
	return confirmedDelegatedAliases, pendingDelegatedAliases, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Seed /////////////////////////////////////////////////////////////////////////////////////////////////////////

// Seed returns the seed of this wallet that is used to generate all of the wallets addresses and private keys.
func (wallet *Wallet) Seed() *seed.Seed {
	return wallet.addressManager.seed
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AddressManager ///////////////////////////////////////////////////////////////////////////////////////////////

// AddressManager returns the manager for the addresses of this wallet.
func (wallet *Wallet) AddressManager() *AddressManager {
	return wallet.addressManager
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ExportState //////////////////////////////////////////////////////////////////////////////////////////////////

// ExportState exports the current state of the wallet to a marshaled version.
func (wallet *Wallet) ExportState() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteBytes(wallet.Seed().Bytes())
	marshalUtil.WriteUint64(wallet.AddressManager().lastAddressIndex)
	marshalUtil.WriteBytes(wallet.assetRegistry.Bytes())
	marshalUtil.WriteBytes(*(*[]byte)(unsafe.Pointer(&wallet.addressManager.spentAddresses)))

	return marshalUtil.Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WaitForTxAcceptance //////////////////////////////////////////////////////////////////////////////////////////

// WaitForTxAcceptance waits for the given tx to be accepted.
func (wallet *Wallet) WaitForTxAcceptance(txID utxo.TransactionID, optionalCtx ...context.Context) (err error) {
	ctx := context.Background()
	if len(optionalCtx) == 1 && optionalCtx[0] != nil {
		ctx = optionalCtx[0]
	}

	ticker := time.NewTicker(wallet.ConfirmationPollInterval)
	timeoutCounter := time.Duration(0)
	for {
		select {
		case <-ctx.Done():
			return errors.Errorf("context cancelled")
		case <-ticker.C:
			timeoutCounter += wallet.ConfirmationPollInterval
			confirmationState, fetchErr := wallet.connector.GetTransactionConfirmationState(txID)
			if fetchErr != nil {
				return fetchErr
			}
			if confirmationState.IsAccepted() {
				return
			}
			if timeoutCounter > wallet.ConfirmationTimeout {
				return errors.Errorf("transaction %s did not confirm within %d seconds", txID.Base58(), wallet.ConfirmationTimeout/time.Second)
			}
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Internal Methods /////////////////////////////////////////////////////////////////////////////////////////////

// waitForBalanceConfirmation waits until the balance of the wallet changes compared to the provided argument.
// (a transaction modifying the wallet balance got confirmed).
func (wallet *Wallet) waitForBalanceConfirmation(prevConfirmedBalance map[devnetvm2.Color]uint64) (err error) {
	timeoutCounter := time.Duration(0)
	for {
		time.Sleep(wallet.ConfirmationPollInterval)
		timeoutCounter += wallet.ConfirmationPollInterval
		if err = wallet.Refresh(); err != nil {
			return
		}
		newConfirmedBalance, _, balanceErr := wallet.Balance()
		if balanceErr != nil {
			err = balanceErr
			return
		}
		if !reflect.DeepEqual(prevConfirmedBalance, newConfirmedBalance) {
			return
		}
		if timeoutCounter > wallet.ConfirmationTimeout {
			return errors.Errorf("confirmed balance did not change within timeout limit (%d)", wallet.ConfirmationTimeout/time.Second)
		}
	}
}

// waitForGovAliasBalanceConfirmation waits until the balance of the confirmed governed aliases changes in the wallet.
// (a tx submitting an alias governance transition is confirmed).
func (wallet *Wallet) waitForGovAliasBalanceConfirmation(preGovAliasBalance map[*devnetvm2.AliasAddress]*devnetvm2.AliasOutput) (err error) {
	for {
		time.Sleep(wallet.ConfirmationPollInterval)
		if err = wallet.Refresh(); err != nil {
			return
		}
		newGovAliasBalance, _, _, _, balanceErr := wallet.AliasBalance()
		if balanceErr != nil {
			err = balanceErr
			return
		}
		if !reflect.DeepEqual(preGovAliasBalance, newGovAliasBalance) {
			return
		}
	}
}

// waitForStateAliasBalanceConfirmation waits until the balance of the state controlled aliases changes in the wallet.
// (a tx submitting an alias state transition is confirmed).
func (wallet *Wallet) waitForStateAliasBalanceConfirmation(preStateAliasBalance map[*devnetvm2.AliasAddress]*devnetvm2.AliasOutput) (err error) {
	for {
		time.Sleep(wallet.ConfirmationPollInterval)

		if err = wallet.Refresh(); err != nil {
			return
		}
		_, newStateAliasBalance, _, _, balanceErr := wallet.AliasBalance()
		if balanceErr != nil {
			err = balanceErr

			return
		}

		if !reflect.DeepEqual(preStateAliasBalance, newStateAliasBalance) {
			return
		}
	}
}

// derivePledgeIDs returns the mana pledge IDs from the provided options.
func (wallet *Wallet) derivePledgeIDs(aIDFromOptions, cIDFromOptions string) (aID, cID identity.ID, err error) {
	// determine pledge IDs
	allowedPledgeNodeIDs, err := wallet.connector.GetAllowedPledgeIDs()
	if err != nil {
		return
	}
	if aIDFromOptions == "" {
		aID, err = mana2.IDFromStr(allowedPledgeNodeIDs[mana2.AccessMana][0])
	} else {
		aID, err = mana2.IDFromStr(aIDFromOptions)
	}
	if err != nil {
		return
	}

	if cIDFromOptions == "" {
		cID, err = mana2.IDFromStr(allowedPledgeNodeIDs[mana2.ConsensusMana][0])
	} else {
		cID, err = mana2.IDFromStr(cIDFromOptions)
	}
	return
}

// findGovernedAliasOutputByAliasID tries to load the output with given alias address from output manager that is governed by this wallet.
func (wallet *Wallet) findGovernedAliasOutputByAliasID(id *devnetvm2.AliasAddress) (res *Output, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	unspentAliasOutputs := wallet.outputManager.UnspentAliasOutputs(false)
	for _, outputIDMap := range unspentAliasOutputs {
		for _, output := range outputIDMap {
			if output.Object.Address().Equals(id) && output.Object.(*devnetvm2.AliasOutput).GetGoverningAddress().Equals(output.Address.Address()) {
				res = output
				return res, nil
			}
		}
	}
	err = errors.Errorf("couldn't find aliasID %s in the wallet that is owned for governance", id.Base58())
	return nil, err
}

// findStateControlledAliasOutputByAliasID tries to load the output with given alias address from output manager that is state controlled by this wallet.
func (wallet *Wallet) findStateControlledAliasOutputByAliasID(id *devnetvm2.AliasAddress) (res *Output, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	unspentAliasOutputs := wallet.outputManager.UnspentAliasOutputs(false)
	for _, outputIDMap := range unspentAliasOutputs {
		for _, output := range outputIDMap {
			if output.Object.Address().Equals(id) && output.Object.(*devnetvm2.AliasOutput).GetStateAddress().Equals(output.Address.Address()) {
				res = output
				return res, nil
			}
		}
	}
	err = errors.Errorf("couldn't find aliasID %s in the wallet that is state controlled by the wallet", id.Base58())
	return nil, err
}

// collectOutputsForFunding tries to collect unspent outputs to fund fundingBalance.
// It may collect pending outputs according to flag.
func (wallet *Wallet) collectOutputsForFunding(fundingBalance map[devnetvm2.Color]uint64, includePending bool, addresses ...address.Address) (OutputsByAddressAndOutputID, error) {
	if fundingBalance == nil {
		return nil, errors.Errorf("can't collect fund: empty fundingBalance provided")
	}

	_ = wallet.outputManager.Refresh()
	if len(addresses) == 0 {
		addresses = wallet.addressManager.Addresses()
	}
	unspentOutputs := wallet.outputManager.UnspentValueOutputs(includePending, addresses...)

	collected := make(map[devnetvm2.Color]uint64)
	outputsToConsume := NewAddressToOutputs()
	numOfCollectedOutputs := 0
	now := time.Now()
	for _, addy := range addresses {
		for outputID, output := range unspentOutputs[addy] {
			if output.Object.Type() == devnetvm2.ExtendedLockedOutputType {
				casted := output.Object.(*devnetvm2.ExtendedLockedOutput)
				if casted.TimeLockedNow(now) || !casted.UnlockAddressNow(now).Equals(addy.Address()) {
					// skip the output because we wouldn't be able to unlock it
					continue
				}
			}
			contributingOutput := false
			output.Object.Balances().ForEach(func(color devnetvm2.Color, balance uint64) bool {
				_, has := fundingBalance[color]
				if has {
					collected[color] += balance
					contributingOutput = true
				}
				return true
			})
			if contributingOutput {
				// store the output in the outputs to use for the transfer
				if _, addressEntryExists := outputsToConsume[addy]; !addressEntryExists {
					outputsToConsume[addy] = make(map[utxo.OutputID]*Output)
				}
				outputsToConsume[addy][outputID] = output
				numOfCollectedOutputs++
				if enoughCollected(collected, fundingBalance) && numOfCollectedOutputs <= devnetvm2.MaxInputCount {
					return outputsToConsume, nil
				}
			}
		}
	}

	if enoughCollected(collected, fundingBalance) && numOfCollectedOutputs > devnetvm2.MaxOutputCount {
		return outputsToConsume, errors.Errorf("failed to collect outputs: %w", ErrTooManyOutputs)
	}

	return nil, errors.Errorf("failed to gather initial funds \n %s, there are only \n %s funds available",
		devnetvm2.NewColoredBalances(fundingBalance).String(),
		devnetvm2.NewColoredBalances(collected).String(),
	)
}

// enoughCollected checks if collected has at least target funds.
func enoughCollected(collected, target map[devnetvm2.Color]uint64) bool {
	for color, balance := range target {
		if collected[color] < balance {
			return false
		}
	}
	return true
}

// buildInputs builds a list of deterministically sorted inputs from the provided OutputsByAddressAndOutputID mapping.
func (wallet *Wallet) buildInputs(addressToIDToOutput OutputsByAddressAndOutputID) devnetvm2.Inputs {
	unsortedInputs := devnetvm2.Inputs{}
	for _, outputIDToOutputMap := range addressToIDToOutput {
		for _, output := range outputIDToOutputMap {
			unsortedInputs = append(unsortedInputs, output.Object.Input())
		}
	}
	return devnetvm2.NewInputs(unsortedInputs...)
}

// buildOutputs builds outputs based on desired destination balances and consumedFunds. If consumedFunds is greater, than
// the destination funds, remainderAddress specifies where the remaining amount is put.
func (wallet *Wallet) buildOutputs(
	sendOptions *sendoptions.SendFundsOptions,
	consumedFunds map[devnetvm2.Color]uint64,
	remainderAddress address.Address,
) (outputs devnetvm2.Outputs) {
	// build outputs for destinations
	outputsByColor := make(map[address.Address]map[devnetvm2.Color]uint64)
	for walletAddress, coloredBalances := range sendOptions.Destinations {
		if _, addressExists := outputsByColor[walletAddress]; !addressExists {
			outputsByColor[walletAddress] = make(map[devnetvm2.Color]uint64)
		}
		for color, amount := range coloredBalances {
			outputsByColor[walletAddress][color] += amount
			if color == devnetvm2.ColorMint {
				consumedFunds[devnetvm2.ColorIOTA] -= amount

				if consumedFunds[devnetvm2.ColorIOTA] == 0 {
					delete(consumedFunds, devnetvm2.ColorIOTA)
				}
			} else {
				consumedFunds[color] -= amount

				if consumedFunds[color] == 0 {
					delete(consumedFunds, color)
				}
			}
		}
	}
	// construct result
	var outputsSlice []devnetvm2.Output

	// add output for remainder
	if len(consumedFunds) != 0 {
		outputsSlice = append(outputsSlice, devnetvm2.NewSigLockedColoredOutput(devnetvm2.NewColoredBalances(consumedFunds), remainderAddress.Address()))
	}

	for addr, outputBalanceMap := range outputsByColor {
		coloredBalances := devnetvm2.NewColoredBalances(outputBalanceMap)
		var output devnetvm2.Output
		if !sendOptions.LockUntil.IsZero() || !sendOptions.FallbackDeadline.IsZero() || sendOptions.FallbackAddress != nil {
			extended := devnetvm2.NewExtendedLockedOutput(outputBalanceMap, addr.Address())
			if !sendOptions.LockUntil.IsZero() {
				extended = extended.WithTimeLock(sendOptions.LockUntil)
			}
			if !sendOptions.FallbackDeadline.IsZero() && sendOptions.FallbackAddress != nil {
				extended = extended.WithFallbackOptions(sendOptions.FallbackAddress, sendOptions.FallbackDeadline)
			}
			output = extended
		} else {
			output = devnetvm2.NewSigLockedColoredOutput(coloredBalances, addr.Address())
		}

		outputsSlice = append(outputsSlice, output)
	}
	outputs = devnetvm2.NewOutputs(outputsSlice...)

	return
}

// buildUnlockBlocks constructs the unlock blocks for a transaction.
func (wallet *Wallet) buildUnlockBlocks(inputs devnetvm2.Inputs, consumedOutputsByID OutputsByID, essence *devnetvm2.TransactionEssence) (unlocks devnetvm2.UnlockBlocks, inputsInOrder devnetvm2.Outputs) {
	unlocks = make([]devnetvm2.UnlockBlock, len(inputs))
	existingUnlockBlocks := make(map[address.Address]uint16)
	for outputIndex, input := range inputs {
		output := consumedOutputsByID[input.(*devnetvm2.UTXOInput).ReferencedOutputID()]
		inputsInOrder = append(inputsInOrder, output.Object)
		if unlockBlockIndex, unlockBlockExists := existingUnlockBlocks[output.Address]; unlockBlockExists {
			unlocks[outputIndex] = devnetvm2.NewReferenceUnlockBlock(unlockBlockIndex)
			continue
		}

		keyPair := wallet.Seed().KeyPair(output.Address.Index)
		unlockBlock := devnetvm2.NewSignatureUnlockBlock(devnetvm2.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(essence.Bytes()))))
		unlocks[outputIndex] = unlockBlock
		existingUnlockBlocks[output.Address] = uint16(outputIndex)
	}
	return
}

// markOutputsAndAddressesSpent marks consumed outputs and their addresses as spent.
func (wallet *Wallet) markOutputsAndAddressesSpent(consumedOutputs OutputsByAddressAndOutputID) {
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
}

// chooseRemainderAddress chooses an appropriate remainder address based on the wallet configuration and where we are spending from.
func (wallet *Wallet) chooseRemainderAddress(consumedOutputs OutputsByAddressAndOutputID, optionsRemainder address.Address) (remainder address.Address) {
	if optionsRemainder == address.AddressEmpty {
		if wallet.reusableAddress {
			return wallet.RemainderAddress()
		}
		_, spendFromRemainderAddress := consumedOutputs[wallet.RemainderAddress()]
		_, spendFromReceiveAddress := consumedOutputs[wallet.ReceiveAddress()]
		if spendFromRemainderAddress && spendFromReceiveAddress {
			// we are about to spend from both
			return wallet.NewReceiveAddress()
		}
		if spendFromRemainderAddress && !spendFromReceiveAddress {
			// we are about to spend from remainder, but not from receive
			return wallet.ReceiveAddress()
		}
		// we are not spending from remainder
		return wallet.RemainderAddress()
	}
	return optionsRemainder
}

// chooseToAddress chooses an appropriate toAddress based on the wallet configuration and where we are spending from.
func (wallet *Wallet) chooseToAddress(consumedOutputs OutputsByAddressAndOutputID, optionsToAddress address.Address) (toAddress address.Address) {
	if optionsToAddress == address.AddressEmpty {
		if wallet.reusableAddress {
			return wallet.ReceiveAddress()
		}
		_, spendFromRemainderAddress := consumedOutputs[wallet.RemainderAddress()]
		_, spendFromReceiveAddress := consumedOutputs[wallet.ReceiveAddress()]
		if spendFromRemainderAddress && spendFromReceiveAddress {
			// we are about to spend from both
			return wallet.NewReceiveAddress()
		}
		if spendFromRemainderAddress && !spendFromReceiveAddress {
			// we are about to spend from remainder, but not from receive
			return wallet.ReceiveAddress()
		}
		// we are not spending from remainder
		return wallet.RemainderAddress()
	}
	return optionsToAddress
}

// checkBalancesAndUnlocks checks if tx balances are okay and unlock blocks are valid.
func checkBalancesAndUnlocks(inputs devnetvm2.Outputs, tx *devnetvm2.Transaction) (bool, error) {
	balancesValid := devnetvm2.TransactionBalancesValid(inputs, tx.Essence().Outputs())
	unlocksValid, err := devnetvm2.UnlockBlocksValidWithError(inputs, tx)
	if err != nil {
		return false, err
	}
	return balancesValid && unlocksValid, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
