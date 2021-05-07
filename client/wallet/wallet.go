package wallet

import (
	"reflect"
	"time"
	"unsafe"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/consolidatefunds_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/createnft_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/delegatefunds_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/depositfundstonft_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/destroynft_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/reclaimfunds_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sendfunds_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownedfunds_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/sweepnftownednfts_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/transfernft_options"
	"github.com/iotaledger/goshimmer/client/wallet/packages/withdrawfundsfromnft_options"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
)

// region Wallet ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Wallet is a wallet that can handle aliases and extendedlockedoutputs.
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SendFunds ////////////////////////////////////////////////////////////////////////////////////////////////////

// SendFunds sends funds from the wallet
func (wallet *Wallet) SendFunds(options ...sendfunds_options.SendFundsOption) (tx *ledgerstate.Transaction, err error) {
	sendOptions, err := sendfunds_options.BuildSendFundsOptions(options...)
	if err != nil {
		return
	}

	// how much funds will we need to fund this transfer?
	requiredFunds := sendOptions.RequiredFunds()
	// collect that many outputs for funding
	consumedOutputs, err := wallet.collectOutputsForFunding(requiredFunds)
	if err != nil {
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
	var remainderAddress address.Address
	if sendOptions.RemainderAddress == address.AddressEmpty {
		_, spendFromRemainderAddress := consumedOutputs[wallet.RemainderAddress()]
		_, spendFromReceiveAddress := consumedOutputs[wallet.ReceiveAddress()]
		if spendFromRemainderAddress && spendFromReceiveAddress {
			// we are about to spend from both
			remainderAddress = wallet.NewReceiveAddress()
		} else if spendFromRemainderAddress && !spendFromReceiveAddress {
			// we are about to spend from remainder, but not from receive
			remainderAddress = wallet.ReceiveAddress()
		} else {
			// we are not spending from remainder
			remainderAddress = wallet.RemainderAddress()
		}
	} else {
		remainderAddress = sendOptions.RemainderAddress
	}
	outputs := wallet.buildOutputs(sendOptions, totalConsumedFunds, remainderAddress)

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), aPledgeID, cPledgeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()

	unlockBlocks, inputsAsOutputsInOrder := wallet.buildUnlockBlocks(inputs, outputsByID, txEssence)

	tx = ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsAsOutputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
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

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}
	if sendOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsolidateFunds ///////////////////////////////////////////////////////////////////////////////////////////////////

func (wallet *Wallet) ConsolidateFunds(options ...consolidatefunds_options.ConsolidateFundsOption) (tx *ledgerstate.Transaction, err error) {
	consolidateOptions, err := consolidatefunds_options.BuildConsolidateFundsOptions(options...)
	if err != nil {
		return
	}
	// get available balances
	confirmedAvailableBalance, _, err := wallet.AvailableBalance()
	if err != nil {
		return
	}
	if len(confirmedAvailableBalance) == 0 {
		err = xerrors.Errorf("no available balance to be swept in wallet")
		return
	}
	// collect outputs
	consumedOutputs, err := wallet.collectOutputsForFunding(confirmedAvailableBalance)
	if err != nil {
		return
	}

	// build inputs from consumed outputs
	inputs := wallet.buildInputs(consumedOutputs)
	// aggregate all the funds we consume from inputs
	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	var toAddress address.Address
	_, spendFromRemainderAddress := consumedOutputs[wallet.RemainderAddress()]
	_, spendFromReceiveAddress := consumedOutputs[wallet.ReceiveAddress()]
	if spendFromRemainderAddress && spendFromReceiveAddress {
		// we are about to spend from both
		toAddress = wallet.NewReceiveAddress()
	} else if spendFromRemainderAddress && !spendFromReceiveAddress {
		// we are about to spend from remainder, but not from receive
		toAddress = wallet.ReceiveAddress()
	} else {
		// we are not spending from remainder
		toAddress = wallet.RemainderAddress()
	}

	outputs := ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(totalConsumedFunds), toAddress.Address()))

	// determine pledgeIDs
	aPledgeID, cPledgeID, err := wallet.derivePledgeIDs(consolidateOptions.AccessManaPledgeID, consolidateOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), aPledgeID, cPledgeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()

	unlockBlocks, inputsAsOutputsInOrder := wallet.buildUnlockBlocks(inputs, outputsByID, txEssence)

	tx = ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsAsOutputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
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

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}
	if consolidateOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CreateAsset //////////////////////////////////////////////////////////////////////////////////////////////////

// CreateAsset creates a new colored token with the given details.
func (wallet *Wallet) CreateAsset(asset Asset, waitForConfirmation ...bool) (assetColor ledgerstate.Color, err error) {
	if asset.Amount == 0 {
		err = xerrors.New("required to provide the amount when trying to create an asset")

		return
	}

	if asset.Name == "" {
		err = xerrors.New("required to provide a name when trying to create an asset")

		return
	}

	// which address to send to? remainder/receive/new receive?
	var receiveAddress address.Address
	// where will we spend from?
	consumedOutputs, err := wallet.collectOutputsForFunding(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: asset.Amount})
	if err != nil {
		return
	}
	_, spendFromRemainderAddress := consumedOutputs[wallet.RemainderAddress()]
	_, spendFromReceiveAddress := consumedOutputs[wallet.ReceiveAddress()]
	if spendFromRemainderAddress && spendFromReceiveAddress {
		// we are about to spend from both
		receiveAddress = wallet.NewReceiveAddress()
	} else if spendFromRemainderAddress && !spendFromReceiveAddress {
		// we are about to spend from remainder, but not from receive
		receiveAddress = wallet.ReceiveAddress()
	} else {
		// we are not spending from remainder
		receiveAddress = wallet.RemainderAddress()
	}

	var wait bool
	if len(waitForConfirmation) > 0 {
		wait = waitForConfirmation[0]
	}

	tx, err := wallet.SendFunds(
		sendfunds_options.Destination(receiveAddress, asset.Amount, ledgerstate.ColorMint),
		sendfunds_options.WaitForConfirmation(wait),
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DelegateFunds ////////////////////////////////////////////////////////////////////////////////////////////////

// DelegateFunds delegates funds to a given address by creating a delegated alias output.
func (wallet *Wallet) DelegateFunds(options ...delegatefunds_options.DelegateFundsOption) (tx *ledgerstate.Transaction, delegationIDs []*ledgerstate.AliasAddress, err error) {
	// build options
	delegateOptions, err := delegatefunds_options.BuildDelegateFundsOptions(options...)
	if err != nil {
		return
	}

	// how much funds will we need to fund this transfer?
	requiredFunds := delegateOptions.RequiredFunds()
	// collect that many outputs for funding
	consumedOutputs, err := wallet.collectOutputsForFunding(requiredFunds)
	if err != nil {
		return
	}

	// determine pledgeIDs
	aPledgeID, cPledgeID, err := wallet.derivePledgeIDs(delegateOptions.AccessManaPledgeID, delegateOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	// build inputs from consumed outputs
	inputs := wallet.buildInputs(consumedOutputs)
	// aggregate all the funds we consume from inputs
	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	var remainderAddress address.Address
	if delegateOptions.RemainderAddress == address.AddressEmpty {
		_, spendFromRemainderAddress := consumedOutputs[wallet.RemainderAddress()]
		_, spendFromReceiveAddress := consumedOutputs[wallet.ReceiveAddress()]
		if spendFromRemainderAddress && spendFromReceiveAddress {
			// we are about to spend from both
			remainderAddress = wallet.NewReceiveAddress()
		} else if spendFromRemainderAddress && !spendFromReceiveAddress {
			// we are about to spend from remainder, but not from receive
			remainderAddress = wallet.ReceiveAddress()
		} else {
			// we are not spending from remainder
			remainderAddress = wallet.RemainderAddress()
		}
	} else {
		remainderAddress = delegateOptions.RemainderAddress
	}

	unsortedOutputs := ledgerstate.Outputs{}
	for addr, balanceMap := range delegateOptions.Destinations {
		var delegationOutput *ledgerstate.AliasOutput
		delegationOutput, err = ledgerstate.NewAliasOutputMint(balanceMap, addr.Address())
		if err != nil {
			return
		}
		// we are the governance controllers, so we can claim back the delegated funds
		delegationOutput.SetGoverningAddress(wallet.ReceiveAddress().Address())
		// is there a delegation timelock?
		if !delegateOptions.DelegateUntil.IsZero() {
			delegationOutput = delegationOutput.WithDelegationAndTimelock(delegateOptions.DelegateUntil)
		} else {
			delegationOutput = delegationOutput.WithDelegation()
		}
		unsortedOutputs = append(unsortedOutputs, delegationOutput)
	}
	// remainder balance = totalConsumed - required
	for color, balance := range requiredFunds {
		if totalConsumedFunds[color] < balance {
			err = xerrors.Errorf("delegated funds are greater than consumed funds")
			return
		}
		totalConsumedFunds[color] -= balance
		if totalConsumedFunds[color] <= 0 {
			delete(totalConsumedFunds, color)
		}
	}
	// only create remainder output if there is a remainder balance
	if len(totalConsumedFunds) > 0 {
		remainderBalances := ledgerstate.NewColoredBalances(totalConsumedFunds)
		unsortedOutputs = append(unsortedOutputs, ledgerstate.NewSigLockedColoredOutput(remainderBalances, remainderAddress.Address()))
	}

	outputs := ledgerstate.NewOutputs(unsortedOutputs...)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), aPledgeID, cPledgeID, inputs, outputs)
	outputsByID := consumedOutputs.OutputsByID()
	unlockBlocks, inputsAsOutputsInOrder := wallet.buildUnlockBlocks(inputs, outputsByID, txEssence)
	tx = ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsAsOutputsInOrder, tx)
	if err != nil {
		return
	}
	if !ok {
		err = xerrors.Errorf("created transaction is invalid: %s", tx.String())
		return
	}

	// look for the ids of the freshly created delegation aliases that are only available after the outputID is set.
	delegationIDs = make([]*ledgerstate.AliasAddress, 0)
	for _, output := range tx.Essence().Outputs() {
		if output.Type() == ledgerstate.AliasOutputType {
			// Address() for an alias output returns the alias address, the unique ID of the alias
			delegationIDs = append(delegationIDs, output.Address().(*ledgerstate.AliasAddress))
		}
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

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return
	}
	if delegateOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReclaimDelegatedFunds ////////////////////////////////////////////////////////////////////////////////////////

// ReclaimDelegatedFunds reclaims delegated funds (alias outputs).
func (wallet *Wallet) ReclaimDelegatedFunds(options ...reclaimfunds_options.ReclaimFundsOption) (tx *ledgerstate.Transaction, err error) {
	// build options
	reclaimOptions, err := reclaimfunds_options.BuildReclaimFundsOptions(options...)
	if err != nil {
		return
	}
	if reclaimOptions.ToAddress == nil {
		// if no optional address is provided, send to receive address of the wallet
		reclaimOptions.ToAddress = wallet.ReceiveAddress().Address()
	}

	tx, err = wallet.DestroyNFT(
		destroynft_options.Alias(reclaimOptions.Alias.Base58()),
		destroynft_options.RemainderAddress(reclaimOptions.ToAddress.Base58()),
		destroynft_options.WaitForConfirmation(reclaimOptions.WaitForConfirmation),
	)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CreateNFT ////////////////////////////////////////////////////////////////////////////////////////////////////

// CreateNFT spends funds from the wallet to create an NFT.
func (wallet *Wallet) CreateNFT(options ...createnft_options.CreateNFTOption) (tx *ledgerstate.Transaction, nftID *ledgerstate.AliasAddress, err error) { // build options from the parameters
	// build options
	createNFTOptions, err := createnft_options.BuildCreateNFTOptions(options...)
	if err != nil {
		return
	}
	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(createNFTOptions.AccessManaPledgeID, createNFTOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}
	// collect funds required for an alias input
	consumedOutputs, err := wallet.collectOutputsForFunding(createNFTOptions.InitialBalance)
	if err != nil {
		return nil, nil, err
	}
	// get a new address from address manager
	nftWalletAddress := wallet.NewReceiveAddress()
	// build inputs from consumed outputs
	inputs := wallet.buildInputs(consumedOutputs)
	// aggregate all the funds we consume from inputs
	totalConsumedFunds := consumedOutputs.TotalFundsInOutputs()
	// create an alias mint output
	nft, err := ledgerstate.NewAliasOutputMint(
		createNFTOptions.InitialBalance,
		nftWalletAddress.Address(),
		createNFTOptions.ImmutableData,
	)
	if err != nil {
		return nil, nil, err
	}
	unsortedOutputs := ledgerstate.Outputs{nft}

	// calculate remainder balances (consumed - nft balance)
	nft.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
		totalConsumedFunds[color] -= balance
		if totalConsumedFunds[color] <= 0 {
			delete(totalConsumedFunds, color)
		}
		return true
	})
	remainderBalances := ledgerstate.NewColoredBalances(totalConsumedFunds)
	// only add remainder output if there is a remainder balance
	if remainderBalances.Size() != 0 {
		unsortedOutputs = append(unsortedOutputs, ledgerstate.NewSigLockedColoredOutput(
			remainderBalances, wallet.addressManager.FirstUnspentAddress().Address()))
	}
	// create tx essence
	outputs := ledgerstate.NewOutputs(unsortedOutputs...)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, inputs, outputs)

	// build unlock blocks
	unlockBlocks, inputsInOrder := wallet.buildUnlockBlocks(inputs, consumedOutputs.OutputsByID(), txEssence)

	tx = ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
	}

	// look for the id of the freshly created nft (alias) that is only available after the outputID is set.
	for _, output := range tx.Essence().Outputs() {
		if output.Type() == ledgerstate.AliasOutputType {
			// Address() for an alias output returns the alias address, the unique ID of the alias
			nftID = output.Address().(*ledgerstate.AliasAddress)
		}
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

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, nil, err
	}
	if createNFTOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransferNFT //////////////////////////////////////////////////////////////////////////////////////////////////

// TransferNFT transfers an NFT to a given address.
func (wallet *Wallet) TransferNFT(options ...transfernft_options.TransferNFTOption) (tx *ledgerstate.Transaction, err error) {
	transferOptions, err := transfernft_options.BuildTransferNFTOptions(options...)
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
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)
	if alias.DelegationTimeLockedNow(time.Now()) {
		err = xerrors.Errorf("alias %s is delegation timelocked until %s", alias.GetAliasAddress().Base58(),
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
	if transferOptions.ToAddress.Type() == ledgerstate.AliasAddressType {
		// we are giving the governor role to another alias. Is that other alias governed by this alias?
		var otherAlias *ledgerstate.AliasOutput
		otherAlias, err = wallet.connector.GetUnspentAliasOutput(transferOptions.ToAddress.(*ledgerstate.AliasAddress))
		if err != nil {
			err = xerrors.Errorf("failed to check that transfer wouldn't result in deadlocked outputs: %w", err)
			return
		}
		if otherAlias.GetGoverningAddress().Equals(alias.GetAliasAddress()) {
			err = xerrors.Errorf("transfer of nft to %s would result in circular alias governance", transferOptions.ToAddress.Base58())
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

	essence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID,
		ledgerstate.NewInputs(alias.Input()),
		ledgerstate.NewOutputs(nextAlias),
	)
	// there is only one input, so signing is easy
	keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
	tx = ledgerstate.NewTransaction(essence, ledgerstate.UnlockBlocks{
		ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essence.Bytes()))),
	})

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(ledgerstate.Outputs{alias}, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
	}

	// mark output as spent
	wallet.outputManager.MarkOutputSpent(walletAlias.Address, walletAlias.Object.ID())
	// mark addresses as spent
	if !wallet.reusableAddress {
		wallet.addressManager.MarkAddressSpent(walletAlias.Address.Index)
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if transferOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DestroyNFT ///////////////////////////////////////////////////////////////////////////////////////////////////

// DestroyNFT destroys the given nft (alias).
func (wallet *Wallet) DestroyNFT(options ...destroynft_options.DestroyNFTOption) (tx *ledgerstate.Transaction, err error) {
	destroyOptions, err := destroynft_options.BuildDestroyNFTOptions(options...)
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
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)

	if alias.DelegationTimeLockedNow(time.Now()) {
		err = xerrors.Errorf("alias %s is delegation timelocked until %s", alias.GetAliasAddress().Base58(), alias.DelegationTimelock().String())
		return
	}

	// can only be destroyed when minimal funds are present (unless it is delegated)
	if !alias.IsDelegated() && !ledgerstate.IsExactDustMinimum(alias.Balances()) {
		withdrawAmount := alias.Balances().Map()
		withdrawAmount[ledgerstate.ColorIOTA] -= ledgerstate.DustThresholdAliasOutputIOTA
		_, err = wallet.WithdrawFundsFromNFT(
			withdrawfundsfromnft_options.Alias(destroyOptions.Alias.Base58()),
			withdrawfundsfromnft_options.Amount(withdrawAmount),
			withdrawfundsfromnft_options.WaitForConfirmation(true),
		)
		if err != nil {
			return
		}
		walletAlias, err = wallet.findGovernedAliasOutputByAliasID(destroyOptions.Alias)
		if err != nil {
			return
		}
		alias = walletAlias.Object.(*ledgerstate.AliasOutput)
	}

	// determine where the remainder will go
	remainderAddy := wallet.ReceiveAddress()
	remainderOutput := ledgerstate.NewSigLockedColoredOutput(alias.Balances(), remainderAddy.Address())

	inputs := ledgerstate.Inputs{alias.Input()}
	outputs := ledgerstate.Outputs{remainderOutput}
	essence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID,
		ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...))

	// there is only one input, so signing is easy
	keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
	tx = ledgerstate.NewTransaction(essence, ledgerstate.UnlockBlocks{
		ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essence.Bytes()))),
	})

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(ledgerstate.Outputs{alias}, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
	}

	// mark output as spent
	wallet.outputManager.MarkOutputSpent(walletAlias.Address, walletAlias.Object.ID())
	// mark addresses as spent
	if !wallet.reusableAddress {
		wallet.addressManager.MarkAddressSpent(walletAlias.Address.Index)
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if destroyOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WithdrawFundsFromNFT /////////////////////////////////////////////////////////////////////////////////////////

// WithdrawFundsFromNFT withdraws funds from the given alias. If the wallet is not the state controller, or too much funds
// are withdrawn, an error is returned.
func (wallet *Wallet) WithdrawFundsFromNFT(options ...withdrawfundsfromnft_options.WithdrawFundsFromNFTOption) (tx *ledgerstate.Transaction, err error) {
	withdrawOptions, err := withdrawfundsfromnft_options.BuildWithdrawFundsFromNFTOptions(options...)
	if err != nil {
		return
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(withdrawOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)
	balancesOfAlias := alias.Balances()
	withdrawBalances := withdrawOptions.Amount
	newAliasBalance := map[ledgerstate.Color]uint64{}

	// check if withdrawBalance is valid for alias
	balancesOfAlias.ForEach(func(color ledgerstate.Color, balance uint64) bool {
		if balance < withdrawBalances[color] {
			err = xerrors.Errorf("trying to withdraw %d %s tokens from alias, but there are only %d tokens in it",
				withdrawBalances[color], color.Base58(), balance)
			return false
		}
		newAliasBalance[color] = balance - withdrawBalances[color]
		if newAliasBalance[color] == 0 {
			delete(newAliasBalance, color)
		}
		if color == ledgerstate.ColorIOTA && newAliasBalance[color] < ledgerstate.DustThresholdAliasOutputIOTA {
			err = xerrors.Errorf("%d IOTA tokens would remain after withdrawal, which is less, then the minimum required %d",
				newAliasBalance[color], ledgerstate.DustThresholdAliasOutputIOTA)
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

	var remainderAddress ledgerstate.Address
	if withdrawOptions.ToAddress == nil {
		remainderAddress = wallet.ReceiveAddress().Address()
	} else {
		remainderAddress = withdrawOptions.ToAddress
	}

	remainderOutput := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(withdrawBalances), remainderAddress)

	inputs := ledgerstate.Inputs{alias.Input()}
	outputs := ledgerstate.Outputs{remainderOutput, nextAlias}

	// derive mana pledge IDs
	accessPledgeNodeID, consensusPledgeNodeID, err := wallet.derivePledgeIDs(withdrawOptions.AccessManaPledgeID, withdrawOptions.ConsensusManaPledgeID)
	if err != nil {
		return
	}

	essence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID,
		ledgerstate.NewInputs(inputs...), ledgerstate.NewOutputs(outputs...))

	// there is only one input, so signing is easy
	keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
	tx = ledgerstate.NewTransaction(essence, ledgerstate.UnlockBlocks{
		ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essence.Bytes()))),
	})

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(ledgerstate.Outputs{alias}, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
	}

	// mark output as spent
	wallet.outputManager.MarkOutputSpent(walletAlias.Address, walletAlias.Object.ID())
	// mark addresses as spent
	if !wallet.reusableAddress {
		wallet.addressManager.MarkAddressSpent(walletAlias.Address.Index)
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if withdrawOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DepositFundsToNFT ////////////////////////////////////////////////////////////////////////////////////////////

// DepositFundsToNFT deposits funds to the given alias from the wallet funds. If the wallet is not the state controller, an error is returned.
func (wallet *Wallet) DepositFundsToNFT(options ...depositfundstonft_options.DepositFundsToNFTOption) (tx *ledgerstate.Transaction, err error) {
	depositOptions, err := depositfundstonft_options.BuildDepositFundsToNFTOptions(options...)
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
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)
	depositBalances := depositOptions.Amount
	newAliasBalance := alias.Balances().Map() // we are going to top it up with depositbalances
	// add deposit balances to alias balance
	for color, balance := range depositBalances {
		newAliasBalance[color] += balance
	}

	// collect funds required for a deposit
	consumedOutputs, err := wallet.collectOutputsForFunding(depositBalances)
	if err != nil {
		return nil, err
	}
	// build inputs from consumed outputs
	inputsFromConsumedOutputs := wallet.buildInputs(consumedOutputs)
	// add the alias
	unsortedInputs := append(inputsFromConsumedOutputs, alias.Input())
	// sort all inputs
	inputs := ledgerstate.NewInputs(unsortedInputs...)
	// aggregate all the funds we consume from inputs used to fund the deposit (there is the alias input as well)
	totalConsumed := consumedOutputs.TotalFundsInOutputs()
	// create the alias state transition (only state transition can modify balance)
	nextAlias := alias.NewAliasOutputNext(false)
	// update the balance of the deposited nft output
	err = nextAlias.SetBalances(newAliasBalance)
	if err != nil {
		return nil, err
	}
	unsortedOutputs := ledgerstate.Outputs{nextAlias}

	// remainder balance = totalConsumed - deposit
	for color, balance := range depositBalances {
		if totalConsumed[color] < balance {
			return nil, xerrors.Errorf("deposit funds are greater than consumed funds")
		}
		totalConsumed[color] -= balance
		if totalConsumed[color] <= 0 {
			delete(totalConsumed, color)
		}
	}
	remainderBalances := ledgerstate.NewColoredBalances(totalConsumed)
	// remainder funds sent here
	remainderAddress := wallet.ReceiveAddress()
	// only add remainder output if there is a remainder balance
	if remainderBalances.Size() != 0 {
		unsortedOutputs = append(unsortedOutputs, ledgerstate.NewSigLockedColoredOutput(
			remainderBalances, remainderAddress.Address()))
	}

	// create tx essence
	outputs := ledgerstate.NewOutputs(unsortedOutputs...)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, inputs, outputs)
	// add the alias to the consumed outputs
	if _, exists := consumedOutputs[walletAlias.Address]; !exists {
		consumedOutputs[walletAlias.Address] = make(map[ledgerstate.OutputID]*Output)
	}
	consumedOutputs[walletAlias.Address][walletAlias.Object.ID()] = walletAlias

	// build unlock blocks
	unlockBlocks, inputsInOrder := wallet.buildUnlockBlocks(inputs, consumedOutputs.OutputsByID(), txEssence)

	tx = ledgerstate.NewTransaction(txEssence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
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

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if depositOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SweepNFTOwnedFunds ///////////////////////////////////////////////////////////////////////////////////////////

// SweepNFTOwnedFunds collects all funds from non-alias outputs that are owned by the nft into the wallet.
func (wallet Wallet) SweepNFTOwnedFunds(options ...sweepnftownedfunds_options.SweepNFTOwnedFundsOption) (tx *ledgerstate.Transaction, err error) {
	sweepOptions, err := sweepnftownedfunds_options.BuildSweepNFTOwnedFundsOptions(options...)
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
	if _, has := stateControlled[*sweepOptions.Alias]; !has {
		err = xerrors.Errorf("nft %s is not state controlled by the wallet", sweepOptions.Alias.Base58())
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(sweepOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)

	owned, _, err := wallet.AvailableOutputsOnNFT(sweepOptions.Alias.Base58())
	if err != nil {
		return
	}
	if len(owned) == 0 {
		err = xerrors.Errorf("no owned outputs with funds are found on nft %s", sweepOptions.Alias.Base58())
	}

	toBeConsumed := ledgerstate.Outputs{}
	totalConsumed := map[ledgerstate.Color]uint64{}
	// owned contains all outputs that are owned by nft. we want to filter out alias outputs, as they are not "funds"
	for _, output := range owned {
		if len(toBeConsumed) == 126 {
			// we can spend at most 127 inputs in a tx, need one more for the alias
			break
		}
		if output.Type() == ledgerstate.AliasOutputType {
			continue
			//casted := output.(*ledgerstate.AliasOutput)
			//if !casted.IsDelegated() && !ledgerstate.IsExactDustMinimum(casted.Balances()) {
			//	// we are trying to destroy an alias that is not delegated and has more funds than minimum
			//	// TODO: withdraw from it
			//	continue
			//}
		}
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			totalConsumed[color] += balance
			return true
		})
		toBeConsumed = append(toBeConsumed, output)
	}

	nextAlias := alias.NewAliasOutputNext(false)
	toBeConsumed = append(toBeConsumed, alias)

	unsortedInputs := toBeConsumed.Inputs()
	unsortedOutputs := ledgerstate.Outputs{nextAlias, ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(totalConsumed), wallet.ReceiveAddress().Address())}

	essence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, ledgerstate.NewInputs(unsortedInputs...), ledgerstate.NewOutputs(unsortedOutputs...))

	toBeConsumeByID := toBeConsumed.ByID()
	inputsInOrder := ledgerstate.Outputs{}
	unlockBlocks := make(ledgerstate.UnlockBlocks, len(essence.Inputs()))
	aliasInputIndex := -1
	// find the input of alias
	for index, input := range essence.Inputs() {
		if input.Type() == ledgerstate.UTXOInputType {
			casted := input.(*ledgerstate.UTXOInput)
			if casted.ReferencedOutputID() == alias.ID() {
				keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
				unlockBlock := ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essence.Bytes())))
				unlockBlocks[index] = unlockBlock
				aliasInputIndex = index
			}
			inputsInOrder = append(inputsInOrder, toBeConsumeByID[casted.ReferencedOutputID()])
		}

	}
	if aliasInputIndex < 0 {
		err = xerrors.Errorf("failed to find alias %s among prepared transaction inputs", alias.GetAliasAddress().Base58())
		return
	}
	// fill rest of the unlock blocks
	for i, _ := range essence.Inputs() {
		if i != aliasInputIndex {
			unlockBlocks[i] = ledgerstate.NewAliasUnlockBlock(uint16(aliasInputIndex))
		}
	}

	tx = ledgerstate.NewTransaction(essence, unlockBlocks)

	// check syntactical validity by marshaling an unmarshaling
	tx, _, err = ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return nil, err
	}

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, xerrors.Errorf("created transaction is invalid: %s", tx.String())
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	if sweepOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SweepNFTOwnedNFTs ////////////////////////////////////////////////////////////////////////////////////////////

func (wallet *Wallet) SweepNFTOwnedNFTs(options ...sweepnftownednfts_options.SweepNFTOwnedNFTsOption) (tx *ledgerstate.Transaction, sweptNFTs []*ledgerstate.AliasAddress, err error) {
	sweepOptions, err := sweepnftownednfts_options.BuildSweepNFTOwnedNFTsOptions(options...)
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
	if _, has := stateControlled[*sweepOptions.Alias]; !has {
		err = xerrors.Errorf("nft %s is not state controlled by the wallet", sweepOptions.Alias.Base58())
	}
	// look up if we have the alias output. Only the state controller can modify balances in aliases.
	walletAlias, err := wallet.findStateControlledAliasOutputByAliasID(sweepOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)
	owned, _, err := wallet.AvailableOutputsOnNFT(sweepOptions.Alias.Base58())
	if err != nil {
		return
	}
	if len(owned) == 0 {
		err = xerrors.Errorf("no owned outputs with funds are found on nft %s", sweepOptions.Alias.Base58())
	}
	toBeConsumed := ledgerstate.Outputs{}
	// owned contains all outputs that are owned by nft. we want to filter out non alias outputs
	now := time.Now()
	for _, output := range owned {
		if len(toBeConsumed) == 126 {
			// we can spend at most 127 inputs in a tx, need one more for the alias
			break
		}
		if output.Type() == ledgerstate.AliasOutputType {
			casted := output.(*ledgerstate.AliasOutput)
			if casted.DelegationTimeLockedNow(now) {
				// the output is delegation timelocked at the moment, so the governor can't move it
				continue
			}
			toBeConsumed = append(toBeConsumed, output)
		}

	}
	// determine which address to send to
	var toAddress ledgerstate.Address
	if sweepOptions.ToAddress != nil {
		toAddress = sweepOptions.ToAddress
	} else {
		toAddress = wallet.ReceiveAddress().Address()
	}
	// nextAlias is the nft we control
	nextAlias := alias.NewAliasOutputNext(false)
	// transition nft owned aliases
	unsortedOutputs := ledgerstate.Outputs{nextAlias}
	for _, output := range toBeConsumed {
		if output.Type() == ledgerstate.AliasOutputType {
			next := output.(*ledgerstate.AliasOutput).NewAliasOutputNext(true)
			// set to self-governed by toAddress
			next.SetGoverningAddress(nil)
			err = next.SetStateAddress(toAddress)
			if err != nil {
				return
			}
			unsortedOutputs = append(unsortedOutputs, next)
		}
	}
	// we will consume the nft that owns the others too
	toBeConsumed = append(toBeConsumed, alias)
	unsortedInputs := toBeConsumed.Inputs()

	// create essence, contains sorted inputs and outputs
	essence := ledgerstate.NewTransactionEssence(0, time.Now(), accessPledgeNodeID, consensusPledgeNodeID, ledgerstate.NewInputs(unsortedInputs...), ledgerstate.NewOutputs(unsortedOutputs...))

	toBeConsumeByID := toBeConsumed.ByID()
	inputsInOrder := ledgerstate.Outputs{}
	unlockBlocks := make(ledgerstate.UnlockBlocks, len(essence.Inputs()))
	aliasInputIndex := -1
	// find the input of alias
	for index, input := range essence.Inputs() {
		if input.Type() == ledgerstate.UTXOInputType {
			casted := input.(*ledgerstate.UTXOInput)
			if casted.ReferencedOutputID() == alias.ID() {
				keyPair := wallet.Seed().KeyPair(walletAlias.Address.Index)
				unlockBlock := ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essence.Bytes())))
				unlockBlocks[index] = unlockBlock
				aliasInputIndex = index
			}
			inputsInOrder = append(inputsInOrder, toBeConsumeByID[casted.ReferencedOutputID()])
		}

	}
	if aliasInputIndex < 0 {
		err = xerrors.Errorf("failed to find alias %s among prepared transaction inputs", alias.GetAliasAddress().Base58())
		return
	}
	// fill rest of the unlock blocks
	for i, _ := range essence.Inputs() {
		if i != aliasInputIndex {
			unlockBlocks[i] = ledgerstate.NewAliasUnlockBlock(uint16(aliasInputIndex))
		}
	}

	tx = ledgerstate.NewTransaction(essence, unlockBlocks)

	//check tx validity (balances, unlock blocks)
	ok, err := checkBalancesAndUnlocks(inputsInOrder, tx)
	if err != nil {
		return
	}
	if !ok {
		err = xerrors.Errorf("created transaction is invalid: %s", tx.String())
		return
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return
	}

	for _, output := range tx.Essence().Outputs() {
		if output.Type() == ledgerstate.AliasOutputType {
			casted := output.(*ledgerstate.AliasOutput)
			if casted.GetAliasAddress().Equals(alias.GetAliasAddress()) {
				// we skip the owned nft
				continue
			}
			sweptNFTs = append(sweptNFTs, casted.GetAliasAddress())
		}
	}

	if sweepOptions.WaitForConfirmation {
		err = wallet.WaitForTxConfirmation(tx.ID())
	}
	return
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
func (wallet *Wallet) AllowedPledgeNodeIDs() (res map[mana.Type][]string, err error) {
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
func (wallet *Wallet) UnspentOutputs() map[address.Address]map[ledgerstate.OutputID]*Output {
	return wallet.outputManager.UnspentOutputs()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnspentValueOutputs //////////////////////////////////////////////////////////////////////////////////////////

// UnspentValueOutputs returns the unspent value type outputs that are available for spending.
func (wallet *Wallet) UnspentValueOutputs() map[address.Address]map[ledgerstate.OutputID]*Output {
	return wallet.outputManager.UnspentValueOutputs()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnspentAliasOutputs //////////////////////////////////////////////////////////////////////////////////////////

// UnspentAliasOutputs returns the unspent alias outputs that are available for spending.
func (wallet *Wallet) UnspentAliasOutputs() map[address.Address]map[ledgerstate.OutputID]*Output {
	return wallet.outputManager.UnspentAliasOutputs()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RequestFaucetFunds ///////////////////////////////////////////////////////////////////////////////////////////

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
func (wallet *Wallet) Balance() (confirmedBalance map[ledgerstate.Color]uint64, pendingBalance map[ledgerstate.Color]uint64, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	confirmedBalance = make(map[ledgerstate.Color]uint64)
	pendingBalance = make(map[ledgerstate.Color]uint64)

	// iterate through the unspent outputs
	for addy, outputsOnAddress := range wallet.outputManager.UnspentOutputs() {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AvailableBalance /////////////////////////////////////////////////////////////////////////////////////////////

// AvailableBalance returns the balance that is not held in aliases, and therefore can be used to fund transfers.
func (wallet *Wallet) AvailableBalance() (confirmedBalance map[ledgerstate.Color]uint64, pendingBalance map[ledgerstate.Color]uint64, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	confirmedBalance = make(map[ledgerstate.Color]uint64)
	pendingBalance = make(map[ledgerstate.Color]uint64)
	now := time.Now()
	// iterate through the unspent outputs
	for addy, outputsOnAddress := range wallet.outputManager.UnspentOutputs() {
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
				if casted.TimeLockedNow(now) {
					// timelocked funds are not available
					continue
				}
				unlockAddyNow := casted.UnlockAddressNow(now)
				if addy.Address().Equals(unlockAddyNow) {
					// we own this output now
					casted.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
						targetMap[color] += balance
						return true
					})
				}
			}
		}
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimelockedBalances ///////////////////////////////////////////////////////////////////////////////////////////

// TimelockedBalances returns all confirmed and pending balances that are currently timelocked.
func (wallet *Wallet) TimelockedBalances() (confirmed, pending TimelockedBalanceSlice, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	confirmed = make([]*TimelockedBalance, 0)
	pending = make([]*TimelockedBalance, 0)
	now := time.Now()

	// iterate through the unspent outputs
	for _, outputsOnAddress := range wallet.outputManager.UnspentOutputs() {
		for _, output := range outputsOnAddress {
			// skip if the output was rejected or spent already
			if output.InclusionState.Spent || output.InclusionState.Rejected {
				continue
			}
			switch output.Object.Type() {
			case ledgerstate.ExtendedLockedOutputType:
				casted := output.Object.(*ledgerstate.ExtendedLockedOutput)
				if casted.TimeLockedNow(now) {
					tBal := &TimelockedBalance{
						Balance:     casted.Balances().Map(),
						LockedUntil: casted.TimeLock(),
					}
					if output.InclusionState.Confirmed {
						confirmed = append(confirmed, tBal)
					} else {
						pending = append(pending, tBal)
					}
				}
			}
		}
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AliasBalance /////////////////////////////////////////////////////////////////////////////////////////////////

// AliasBalance returns the aliases held by this wallet
func (wallet *Wallet) AliasBalance() (
	confirmedGovernedAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	confirmedStateControlledAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	pendingGovernedAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	pendingStateControlledAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	err error,
) {
	confirmedGovernedAliases = map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	confirmedStateControlledAliases = map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	pendingGovernedAliases = map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	pendingStateControlledAliases = map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	err = wallet.Refresh(true)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	aliasOutputs := wallet.UnspentAliasOutputs()

	for addr, outputIDToOutputMap := range aliasOutputs {
		for _, output := range outputIDToOutputMap {
			if output.Object.Type() == ledgerstate.AliasOutputType {
				// skip if the output was rejected or spent already
				if output.InclusionState.Spent || output.InclusionState.Rejected {
					continue
				}
				// target maps
				var governedAliases, stateControlledAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput
				if output.InclusionState.Confirmed {
					governedAliases = confirmedGovernedAliases
					stateControlledAliases = confirmedStateControlledAliases
				} else {
					governedAliases = pendingGovernedAliases
					stateControlledAliases = pendingStateControlledAliases
				}
				alias := output.Object.(*ledgerstate.AliasOutput)
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
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AvailableOutputsOnNFT ////////////////////////////////////////////////////////////////////////////////////////

// AvailableOutputsOnNFT returns all outputs that are either owned (SigLocked***, Extended, stateControlled Alias) or governed
// (governance controlled alias outputs) and are not currently locked.
func (wallet Wallet) AvailableOutputsOnNFT(nftID string) (owned ledgerstate.Outputs, governed ledgerstate.Outputs, err error) {
	aliasAddress, err := ledgerstate.AliasAddressFromBase58EncodedString(nftID)
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
		case ledgerstate.SigLockedSingleOutputType, ledgerstate.SigLockedColoredOutputType:
			owned = append(owned, o)
		case ledgerstate.ExtendedLockedOutputType:
			casted := o.(*ledgerstate.ExtendedLockedOutput)
			if casted.UnlockAddressNow(now).Equals(aliasAddress) && !casted.TimeLockedNow(now) {
				owned = append(owned, o)
			}
		case ledgerstate.AliasOutputType:
			casted := o.(*ledgerstate.AliasOutput)
			// the alias output of aliasAddress is filtered out
			if casted.GetStateAddress().Equals(aliasAddress) && !casted.DelegationTimeLockedNow(now) {
				owned = append(owned, o)
			} else if casted.GetGoverningAddress().Equals(aliasAddress) && !casted.DelegationTimeLockedNow(now) {
				governed = append(governed, o)
			}
		}
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DelegatedAliasBalance ////////////////////////////////////////////////////////////////////////////////////////

// DelegatedAliasBalance returns the pending and confirmed aliases that are delegated.
func (wallet *Wallet) DelegatedAliasBalance() (
	confirmedDelegatedAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	pendingDelegatedAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	err error,
) {
	confirmedDelegatedAliases = map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	pendingDelegatedAliases = map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}

	err = wallet.Refresh(true)
	if err != nil {
		return nil, nil, err
	}

	aliasOutputs := wallet.UnspentAliasOutputs()

	for addr, outputIDToOutputMap := range aliasOutputs {
		for _, output := range outputIDToOutputMap {
			if output.Object.Type() == ledgerstate.AliasOutputType {
				alias := output.Object.(*ledgerstate.AliasOutput)
				// skip if the output was rejected, spent already or not a delegated one
				if output.InclusionState.Spent || output.InclusionState.Rejected || !alias.IsDelegated() {
					continue
				}
				// target maps
				var delegatedAliases map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput
				if output.InclusionState.Confirmed {
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
	}
	return
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

// region WaitForTxConfirmation ////////////////////////////////////////////////////////////////////////////////////////

// WaitForTxConfirmation waits for the given tx to confirm. If the transaction is rejected, an error is returned.
func (wallet *Wallet) WaitForTxConfirmation(txID ledgerstate.TransactionID) (err error) {
	for {
		time.Sleep(500 * time.Millisecond)
		state, fetchErr := wallet.connector.GetTransactionInclusionState(txID)
		if fetchErr != nil {
			return fetchErr
		}
		if state == ledgerstate.Confirmed {
			return
		}
		if state == ledgerstate.Rejected {
			return xerrors.Errorf("transaction %s has been rejected", txID.Base58())
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Internal Methods /////////////////////////////////////////////////////////////////////////////////////////////

// waitForBalanceConfirmation waits until the balance of the wallet changes compared to the provided argument.
// (a transaction modifying the wallet balance got confirmed)
func (wallet *Wallet) waitForBalanceConfirmation(prevConfirmedBalance map[ledgerstate.Color]uint64) (err error) {
	// TODO: sensible timeout limit
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
		if !reflect.DeepEqual(prevConfirmedBalance, newConfirmedBalance) {
			return
		}
	}
}

// waitForGovAliasBalanceConfirmation waits until the balance of the confirmed governed aliases changes in the wallet.
// (a tx submitting an alias governance transition is confirmed)
func (wallet *Wallet) waitForGovAliasBalanceConfirmation(preGovAliasBalance map[*ledgerstate.AliasAddress]*ledgerstate.AliasOutput) (err error) {
	for {
		time.Sleep(500 * time.Millisecond)
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
// (a tx submitting an alias state transition is confirmed)
func (wallet *Wallet) waitForStateAliasBalanceConfirmation(preStateAliasBalance map[*ledgerstate.AliasAddress]*ledgerstate.AliasOutput) (err error) {
	for {
		time.Sleep(500 * time.Millisecond)

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
		aID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		aID, err = mana.IDFromStr(aIDFromOptions)
	}
	if err != nil {
		return
	}

	if cIDFromOptions == "" {
		cID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		cID, err = mana.IDFromStr(cIDFromOptions)
	}
	return
}

// findGovernedAliasOutputByAliasID tries to load the output with given alias address from output manager that is governed by this wallet.
func (wallet *Wallet) findGovernedAliasOutputByAliasID(ID *ledgerstate.AliasAddress) (res *Output, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	unspentAliasOutputs := wallet.outputManager.UnspentAliasOutputs()
	for _, outputIDMap := range unspentAliasOutputs {
		for _, output := range outputIDMap {
			if output.Object.Address().Equals(ID) && output.Object.(*ledgerstate.AliasOutput).GetGoverningAddress().Equals(output.Address.Address()) {
				res = output
				return res, nil
			}
		}
	}
	err = xerrors.Errorf("couldn't find aliasID %s in the wallet that is owned for governance", ID.Base58())
	return nil, err
}

// findStateControlledAliasOutputByAliasID tries to load the output with given alias address from output manager that is state controlled by this wallet.
func (wallet *Wallet) findStateControlledAliasOutputByAliasID(ID *ledgerstate.AliasAddress) (res *Output, err error) {
	err = wallet.outputManager.Refresh()
	if err != nil {
		return
	}

	unspentAliasOutputs := wallet.outputManager.UnspentAliasOutputs()
	for _, outputIDMap := range unspentAliasOutputs {
		for _, output := range outputIDMap {
			if output.Object.Address().Equals(ID) && output.Object.(*ledgerstate.AliasOutput).GetStateAddress().Equals(output.Address.Address()) {
				res = output
				return res, nil
			}
		}
	}
	err = xerrors.Errorf("couldn't find aliasID %s in the wallet that is state controlled by the wallet", ID.Base58())
	return nil, err
}

// collectOutputsForFunding tries to collect unspent outputs to fund fundingBalance
func (wallet *Wallet) collectOutputsForFunding(fundingBalance map[ledgerstate.Color]uint64) (OutputsByAddressAndOutputID, error) {
	if fundingBalance == nil {
		return nil, xerrors.Errorf("can't collect fund: empty fundingBalance provided")
	}

	_ = wallet.outputManager.Refresh()
	addresses := wallet.addressManager.Addresses()
	unspentOutputs := wallet.outputManager.UnspentValueOutputs(addresses...)

	collected := make(map[ledgerstate.Color]uint64)
	outputsToConsume := NewAddressToOutputs()
	now := time.Now()
	for _, addy := range addresses {
		for outputID, output := range unspentOutputs[addy] {
			if output.InclusionState.Spent {
				// skip counting spent outputs
				continue
			}
			if output.Object.Type() == ledgerstate.ExtendedLockedOutputType {
				casted := output.Object.(*ledgerstate.ExtendedLockedOutput)
				if casted.TimeLockedNow(now) || !casted.UnlockAddressNow(now).Equals(addy.Address()) {
					// skip the output because we wouldn't be able to unlock it
					continue
				}
			}
			contributingOutput := false
			output.Object.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
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
					outputsToConsume[addy] = make(map[ledgerstate.OutputID]*Output)
				}
				outputsToConsume[addy][outputID] = output
				if enoughCollected(collected, fundingBalance) {
					return outputsToConsume, nil
				}
			}
		}
	}

	return nil, xerrors.Errorf("failed to gather initial funds \n %s, there are only \n %s funds available",
		ledgerstate.NewColoredBalances(fundingBalance).String(),
		ledgerstate.NewColoredBalances(collected).String(),
	)
}

// enoughCollected checks if collected has at least target funds
func enoughCollected(collected map[ledgerstate.Color]uint64, target map[ledgerstate.Color]uint64) bool {
	for color, balance := range target {
		if collected[color] < balance {
			return false
		}
	}
	return true
}

// buildInputs builds a list of deterministically sorted inputs from the provided OutputsByAddressAndOutputID mapping.
func (wallet *Wallet) buildInputs(addressToIDToOutput OutputsByAddressAndOutputID) ledgerstate.Inputs {
	unsortedInputs := ledgerstate.Inputs{}
	for _, outputIDToOutputMap := range addressToIDToOutput {
		for _, output := range outputIDToOutputMap {
			unsortedInputs = append(unsortedInputs, output.Object.Input())
		}
	}
	return ledgerstate.NewInputs(unsortedInputs...)
}

// buildOutputs builds outputs based on desired destination balances and consumedFunds. If consumedFunds is greater, than
// the destination funds, remainderAddress specifies where the remaining amount is put.
func (wallet *Wallet) buildOutputs(
	sendOptions *sendfunds_options.SendFundsOptions,
	consumedFunds map[ledgerstate.Color]uint64,
	remainderAddress address.Address,
) (outputs ledgerstate.Outputs) {
	// build outputs for destinations
	outputsByColor := make(map[address.Address]map[ledgerstate.Color]uint64)
	for walletAddress, coloredBalances := range sendOptions.Destinations {
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
	// construct result
	var outputsSlice []ledgerstate.Output

	// add output for remainder
	if len(consumedFunds) != 0 {
		outputsSlice = append(outputsSlice, ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(consumedFunds), remainderAddress.Address()))
	}

	for addr, outputBalanceMap := range outputsByColor {
		coloredBalances := ledgerstate.NewColoredBalances(outputBalanceMap)
		var output ledgerstate.Output
		if !sendOptions.LockUntil.IsZero() || !sendOptions.FallbackDeadline.IsZero() || sendOptions.FallbackAddress != nil {
			extended := ledgerstate.NewExtendedLockedOutput(outputBalanceMap, addr.Address())
			if !sendOptions.LockUntil.IsZero() {
				extended = extended.WithTimeLock(sendOptions.LockUntil)
			}
			if !sendOptions.FallbackDeadline.IsZero() && sendOptions.FallbackAddress != nil {
				extended = extended.WithFallbackOptions(sendOptions.FallbackAddress, sendOptions.FallbackDeadline)
			}
			output = extended
		} else {
			output = ledgerstate.NewSigLockedColoredOutput(coloredBalances, addr.Address())
		}

		outputsSlice = append(outputsSlice, output)
	}
	outputs = ledgerstate.NewOutputs(outputsSlice...)

	return
}

// buildUnlockBlocks constructs the unlock blocks for a transaction.
func (wallet *Wallet) buildUnlockBlocks(inputs ledgerstate.Inputs, consumedOutputsByID OutputsByID, essence *ledgerstate.TransactionEssence) (unlocks ledgerstate.UnlockBlocks, inputsInOrder ledgerstate.Outputs) {
	unlocks = make([]ledgerstate.UnlockBlock, len(inputs))
	existingUnlockBlocks := make(map[address.Address]uint16)
	for outputIndex, input := range inputs {
		output := consumedOutputsByID[input.(*ledgerstate.UTXOInput).ReferencedOutputID()]
		inputsInOrder = append(inputsInOrder, output.Object)
		if unlockBlockIndex, unlockBlockExists := existingUnlockBlocks[output.Address]; unlockBlockExists {
			unlocks[outputIndex] = ledgerstate.NewReferenceUnlockBlock(unlockBlockIndex)
			continue
		}

		keyPair := wallet.Seed().KeyPair(output.Address.Index)
		unlockBlock := ledgerstate.NewSignatureUnlockBlock(ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(essence.Bytes())))
		unlocks[outputIndex] = unlockBlock
		existingUnlockBlocks[output.Address] = uint16(outputIndex)
	}
	return
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
