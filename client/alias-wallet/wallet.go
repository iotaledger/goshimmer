package alias_wallet

import (
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/createnft_options"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/destroynft_options"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/seed"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/sendfunds_options"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/transfernft_options"
	"github.com/iotaledger/goshimmer/client/alias-wallet/packages/withdrawfundsfromnft_options"
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
func (wallet *Wallet) SendFunds(options ...sendfunds_options.SendFundsOption) (tx *ledgerstate.Transaction, err error) {
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
func (wallet *Wallet) CreateNFT(options ...createnft_options.CreateNFTOption) (tx *ledgerstate.Transaction, nftID *ledgerstate.AliasAddress, err error) { // build options from the parameters
	createNFTOptions, err := createnft_options.BuildCreateNFTOptions(options...)
	if err != nil {
		return
	}

	// determine pledge IDs
	allowedPledgeNodeIDs, err := wallet.connector.GetAllowedPledgeIDs()
	if err != nil {
		return
	}
	var accessPledgeNodeID identity.ID
	if createNFTOptions.AccessManaPledgeID == "" {
		accessPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		accessPledgeNodeID, err = mana.IDFromStr(createNFTOptions.AccessManaPledgeID)
	}
	if err != nil {
		return
	}

	var consensusPledgeNodeID identity.ID
	if createNFTOptions.ConsensusManaPledgeID == "" {
		consensusPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		consensusPledgeNodeID, err = mana.IDFromStr(createNFTOptions.ConsensusManaPledgeID)
	}
	if err != nil {
		return
	}

	// collect funds required for an alias input
	consumedOutputs, err := wallet.collectOutputsForAliasMint(createNFTOptions.InitialBalance)
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
	// determine unlock blocks
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

	prevGovernedAliases, _, err := wallet.AliasBalance()
	if err != nil {
		return nil, nil, err
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

	for _, output := range tx.Essence().Outputs() {
		if output.Type() == ledgerstate.AliasOutputType {
			// Address() for an alias output returns the alias address, the unique ID of the alias
			nftID = output.Address().(*ledgerstate.AliasAddress)
		}
	}

	if !createNFTOptions.WaitForConfirmation {
		// send transaction
		err = wallet.connector.SendTransaction(tx)
		return
	}

	// else we wait for confirmation

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, nil, err
	}

	for {
		time.Sleep(500 * time.Millisecond)

		if err = wallet.Refresh(); err != nil {
			return
		}
		newGovernedAliases, _, balanceErr := wallet.AliasBalance()
		if balanceErr != nil {
			err = balanceErr

			return
		}

		if !reflect.DeepEqual(prevGovernedAliases, newGovernedAliases) {
			return
		}
	}
}

// TransferNFT transfers an NFT to a given address.
func (wallet *Wallet) TransferNFT(options ...transfernft_options.TransferNFTOption) (tx *ledgerstate.Transaction, err error) {
	transferOptions, err := transfernft_options.BuildTransferNFTOptions(options...)
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
	nextAlias := alias.NewAliasOutputNext(true)
	if nextAlias.IsSelfGoverned() {
		err = nextAlias.SetStateAddress(transferOptions.ToAddress)
		if err != nil {
			return
		}
	} else {
		nextAlias.SetGoverningAddress(transferOptions.ToAddress)
	}

	// determine pledge IDs
	allowedPledgeNodeIDs, err := wallet.connector.GetAllowedPledgeIDs()
	if err != nil {
		return
	}
	var accessPledgeNodeID identity.ID
	if transferOptions.AccessManaPledgeID == "" {
		accessPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		accessPledgeNodeID, err = mana.IDFromStr(transferOptions.AccessManaPledgeID)
	}
	if err != nil {
		return
	}

	var consensusPledgeNodeID identity.ID
	if transferOptions.ConsensusManaPledgeID == "" {
		consensusPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		consensusPledgeNodeID, err = mana.IDFromStr(transferOptions.ConsensusManaPledgeID)
	}
	if err != nil {
		return
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

	prevGovernedAliases, _, err := wallet.AliasBalance()
	if err != nil {
		return nil, err
	}

	// mark output as spent
	wallet.outputManager.MarkOutputSpent(walletAlias.Address, walletAlias.Object.ID())
	// mark addresses as spent
	if !wallet.reusableAddress {
		wallet.addressManager.MarkAddressSpent(walletAlias.Address.Index)
	}

	if !transferOptions.WaitForConfirmation {
		// send transaction
		err = wallet.connector.SendTransaction(tx)
		return
	}

	// else we wait for confirmation

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	for {
		time.Sleep(500 * time.Millisecond)

		if err = wallet.Refresh(); err != nil {
			return
		}
		newGovernedAliases, _, balanceErr := wallet.AliasBalance()
		if balanceErr != nil {
			err = balanceErr

			return
		}

		if !reflect.DeepEqual(prevGovernedAliases, newGovernedAliases) {
			return
		}
	}

	return
}

// DestroyNFT destroys the given nft (alias).
func (wallet *Wallet) DestroyNFT(options ...destroynft_options.DestroyNFTOption) (tx *ledgerstate.Transaction, err error) {
	destroyOptions, err := destroynft_options.BuildDestroyNFTOptions(options...)
	if err != nil {
		return
	}
	// look up if we have the alias output
	walletAlias, err := wallet.findGovernedAliasOutputByAliasID(destroyOptions.Alias)
	if err != nil {
		return
	}
	alias := walletAlias.Object.(*ledgerstate.AliasOutput)

	// can only be destroyed when minimal funds are present
	if !ledgerstate.IsExactDustMinimum(alias.Balances()) {
		err = xerrors.Errorf("alias %s has more, than minimum balances required for destroy transition", alias.GetAliasAddress().Base58())
		return
		// TODO: withdraw funds from alias and continue
	}

	// determine where the remainder will go
	remainderAddy := wallet.ReceiveAddress()
	remainderOutput := ledgerstate.NewSigLockedColoredOutput(alias.Balances(), remainderAddy.Address())

	inputs := ledgerstate.Inputs{alias.Input()}
	outputs := ledgerstate.Outputs{remainderOutput}

	// determine pledge IDs
	allowedPledgeNodeIDs, err := wallet.connector.GetAllowedPledgeIDs()
	if err != nil {
		return
	}
	var accessPledgeNodeID identity.ID
	if destroyOptions.AccessManaPledgeID == "" {
		accessPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		accessPledgeNodeID, err = mana.IDFromStr(destroyOptions.AccessManaPledgeID)
	}
	if err != nil {
		return
	}

	var consensusPledgeNodeID identity.ID
	if destroyOptions.ConsensusManaPledgeID == "" {
		consensusPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		consensusPledgeNodeID, err = mana.IDFromStr(destroyOptions.ConsensusManaPledgeID)
	}
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

	// else we wait for confirmation
	prevGovernedAliases, _, err := wallet.AliasBalance()
	if err != nil {
		return nil, err
	}

	// mark output as spent
	wallet.outputManager.MarkOutputSpent(walletAlias.Address, walletAlias.Object.ID())
	// mark addresses as spent
	if !wallet.reusableAddress {
		wallet.addressManager.MarkAddressSpent(walletAlias.Address.Index)
	}

	if !destroyOptions.WaitForConfirmation {
		// send transaction
		err = wallet.connector.SendTransaction(tx)
		return
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	for {
		time.Sleep(500 * time.Millisecond)

		if err = wallet.Refresh(); err != nil {
			return
		}
		newGovernedAliases, _, balanceErr := wallet.AliasBalance()
		if balanceErr != nil {
			err = balanceErr

			return
		}

		if !reflect.DeepEqual(prevGovernedAliases, newGovernedAliases) {
			return
		}
	}

	return
}

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

	// determine pledge IDs
	allowedPledgeNodeIDs, err := wallet.connector.GetAllowedPledgeIDs()
	if err != nil {
		return
	}
	var accessPledgeNodeID identity.ID
	if withdrawOptions.AccessManaPledgeID == "" {
		accessPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		accessPledgeNodeID, err = mana.IDFromStr(withdrawOptions.AccessManaPledgeID)
	}
	if err != nil {
		return
	}

	var consensusPledgeNodeID identity.ID
	if withdrawOptions.ConsensusManaPledgeID == "" {
		consensusPledgeNodeID, err = mana.IDFromStr(allowedPledgeNodeIDs[mana.AccessMana][0])
	} else {
		consensusPledgeNodeID, err = mana.IDFromStr(withdrawOptions.ConsensusManaPledgeID)
	}
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
	// else we wait for confirmation
	_, prevStateControlledAliases, err := wallet.AliasBalance()
	if err != nil {
		return nil, err
	}

	// mark output as spent
	wallet.outputManager.MarkOutputSpent(walletAlias.Address, walletAlias.Object.ID())
	// mark addresses as spent
	if !wallet.reusableAddress {
		wallet.addressManager.MarkAddressSpent(walletAlias.Address.Index)
	}

	if !withdrawOptions.WaitForConfirmation {
		// send transaction
		err = wallet.connector.SendTransaction(tx)
		return
	}

	err = wallet.connector.SendTransaction(tx)
	if err != nil {
		return nil, err
	}

	for {
		time.Sleep(500 * time.Millisecond)

		if err = wallet.Refresh(); err != nil {
			return
		}
		_, newStateControlledAliases, balanceErr := wallet.AliasBalance()
		if balanceErr != nil {
			err = balanceErr

			return
		}

		if !reflect.DeepEqual(prevStateControlledAliases, newStateControlledAliases) {
			return
		}
	}

	return
}

// AliasBalance returns the aliases held by this wallet
func (wallet *Wallet) AliasBalance() (
	governedAliases map[*ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	stateControlledAliases map[*ledgerstate.AliasAddress]*ledgerstate.AliasOutput,
	err error,
) {
	governedAliases = map[*ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	stateControlledAliases = map[*ledgerstate.AliasAddress]*ledgerstate.AliasOutput{}
	err = wallet.Refresh(true)
	if err != nil {
		return nil, nil, err
	}

	aliasOutputs := wallet.UnspentAliasOutputs()

	for addr, outputIDToOutputMap := range aliasOutputs {
		for _, output := range outputIDToOutputMap {
			if output.Object.Type() == ledgerstate.AliasOutputType {
				alias := output.Object.(*ledgerstate.AliasOutput)
				if alias.GetGoverningAddress().Equals(addr.Address()) {
					// alias is governed by the wallet
					governedAliases[alias.GetAliasAddress()] = alias
				}
				if alias.GetStateAddress().Equals(addr.Address()) {
					// alias is state controlled by the wallet
					stateControlledAliases[alias.GetAliasAddress()] = alias
				}
			}
		}
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

// collectOutputsForAliasMint tries to collect unspent outputs to fund minting an alias.
func (wallet *Wallet) collectOutputsForAliasMint(initialBalance map[ledgerstate.Color]uint64) (OutputsByAddressAndOutputID, error) {
	if initialBalance == nil {
		initialBalance = map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: ledgerstate.DustThresholdAliasOutputIOTA}
	}

	_ = wallet.outputManager.Refresh()
	addresses := wallet.addressManager.Addresses()
	unspentOutputs := wallet.outputManager.UnspentValueOutputs(addresses...)

	collected := make(map[ledgerstate.Color]uint64)
	outputsToConsume := NewAddressToOutputs()
	for i := wallet.addressManager.firstUnspentAddressIndex; i <= wallet.addressManager.lastAddressIndex; i++ {
		addy := wallet.addressManager.Address(i)
		for outputID, output := range unspentOutputs[addy] {
			contributingOutput := false
			output.Object.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				_, has := initialBalance[color]
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
				if enoughCollected(collected, initialBalance) {
					return outputsToConsume, nil
				}
			}
		}
	}

	return nil, xerrors.Errorf("failed to gather initial funds \n %s, there are only \n %s funds available",
		ledgerstate.NewColoredBalances(initialBalance).String(),
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
