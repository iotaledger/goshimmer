package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/iotaledger/hive.go/identity"
)

func TestDependenciesConfirmed(t *testing.T) {
	eligibleEventTriggered := false
	tangle := newTestTangle()
	defer tangle.Shutdown()
	wallets, walletsByAddress, assets := setupEligibilityTests(t, tangle)
	scenarioMessagesApproveEmptyID(t, tangle, wallets, walletsByAddress, assets)

	tangle.EligibilityManager.Events.MessageEligible.Attach(events.NewClosure(func(messageID MessageID) {
		assert.Equal(t, assets.messages["1"].ID(), messageID)
		eligibleEventTriggered = true
	}))

	mockUTXO := ledgerstate.NewUtxoDagMock(t, tangle.LedgerState.UTXODAG)
	tangle.LedgerState.UTXODAG = mockUTXO
	mockUTXO.On("InclusionState", assets.transactions["0"].ID()).Return(ledgerstate.Pending)

	isEligibleFlag := runCheckEligibilityAndGetEligibility(t, tangle, assets.messages["1"].ID())
	assert.False(t, isEligibleFlag, "Message 1 shouldn't be eligible")

	// reset mock, since calls can't be overridden
	mockUTXO.ExpectedCalls = make([]*mock.Call, 0)

	mockUTXO.On("InclusionState", assets.transactions["0"].ID()).Return(ledgerstate.Confirmed)
	isEligibleFlag = runCheckEligibilityAndGetEligibility(t, tangle, assets.messages["1"].ID())
	assert.True(t, isEligibleFlag, "Message 1 isn't eligible")

	assert.True(t, eligibleEventTriggered, "Eligibility event wasn't triggered")
}

func TestDataMessageAlwaysEligible(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	message := newTestDataMessage("data")
	tangle.Storage.StoreMessage(message)

	err := tangle.EligibilityManager.checkEligibility(message.ID())
	assert.NoError(t, err)

	var eligibilityResult bool
	tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		eligibilityResult = messageMetadata.IsEligible()
	})
	assert.True(t, eligibilityResult, "Data messages should always be eligible")
}

func TestDependencyDirectApproval(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	wallets, walletsByAddress, assets := setupEligibilityTests(t, tangle)

	scenarioMessagesApproveDependency(t, tangle, wallets, walletsByAddress, assets)

	mockUTXO := ledgerstate.NewUtxoDagMock(t, tangle.LedgerState.UTXODAG)
	tangle.LedgerState.UTXODAG = mockUTXO
	mockUTXO.On("InclusionState", assets.transactions["0"].ID()).Return(ledgerstate.Pending)

	isEligibleFlag := runCheckEligibilityAndGetEligibility(t, tangle, assets.messages["1"].ID())
	assert.True(t, isEligibleFlag, "Message 1 isn't eligible")
}

func TestUpdateEligibilityAfterDependencyConfirmation(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	wallets, walletsByAddress, assets := setupEligibilityTests(t, tangle)
	txID := assets.transactions["0"].ID()
	eligibleEventTriggered := false

	scenarioMessagesApproveEmptyID(t, tangle, wallets, walletsByAddress, assets)

	tangle.EligibilityManager.Events.MessageEligible.Attach(events.NewClosure(func(messageID MessageID) {
		assert.Equal(t, assets.messages["1"].ID(), messageID)
		eligibleEventTriggered = true
	}))

	mockUTXO := ledgerstate.NewUtxoDagMock(t, tangle.LedgerState.UTXODAG)
	tangle.LedgerState.UTXODAG = mockUTXO
	mockUTXO.On("InclusionState", txID).Return(ledgerstate.Pending)

	messageID := assets.messages["1"].ID()
	isEligibleFlag := runCheckEligibilityAndGetEligibility(t, tangle, messageID)
	assert.False(t, isEligibleFlag, "Message 1 shouldn't be eligible")

	// reset mock, since calls can't be overridden
	mockUTXO.ExpectedCalls = make([]*mock.Call, 0)
	mockUTXO.On("InclusionState", txID).Return(ledgerstate.Confirmed)

	err := tangle.EligibilityManager.updateEligibilityAfterDependencyConfirmation(&txID)
	assert.NoError(t, err)

	tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.True(t, isEligibleFlag, "Message 1 isn't eligible")

	assert.True(t, eligibleEventTriggered, "eligibility event wasn't triggered")
}

func TestDoNotUpdateEligibilityAfterPartialDependencyConfirmation(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	wallets, walletsByAddress, assets := setupEligibilityTests(t, tangle)

	scenarioMoreThanOneDependency(t, tangle, wallets, walletsByAddress, assets)

	mockUTXO := ledgerstate.NewUtxoDagMock(t, tangle.LedgerState.UTXODAG)
	tangle.LedgerState.UTXODAG = mockUTXO
	tx1ID := assets.transactions["1"].ID()
	tx2ID := assets.transactions["2"].ID()
	mockUTXO.On("InclusionState", tx1ID).Return(ledgerstate.Pending)
	mockUTXO.On("InclusionState", tx2ID).Return(ledgerstate.Pending)

	messageID := assets.messages["3"].ID()
	isEligibleFlag := runCheckEligibilityAndGetEligibility(t, tangle, messageID)
	assert.False(t, isEligibleFlag)

	// reset mock, since calls can't be overridden
	mockUTXO.ExpectedCalls = make([]*mock.Call, 0)
	mockUTXO.On("InclusionState", tx1ID).Return(ledgerstate.Confirmed)
	mockUTXO.On("InclusionState", tx2ID).Return(ledgerstate.Pending)

	err := tangle.EligibilityManager.updateEligibilityAfterDependencyConfirmation(&tx1ID)
	assert.NoError(t, err)
	tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.False(t, isEligibleFlag)

	// reset mock, since calls can't be overridden
	mockUTXO.ExpectedCalls = make([]*mock.Call, 0)
	mockUTXO.On("InclusionState", tx1ID).Return(ledgerstate.Confirmed)
	mockUTXO.On("InclusionState", tx2ID).Return(ledgerstate.Confirmed)

	err = tangle.EligibilityManager.updateEligibilityAfterDependencyConfirmation(&tx2ID)
	assert.NoError(t, err)
	tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.True(t, isEligibleFlag)
}

func TestConfirmationMakeEligibleOneOfDependentTransaction(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()
	wallets, walletsByAddress, assets := setupEligibilityTests(t, tangle)
	scenarioMoreThanOneDependentTransaction(t, tangle, wallets, walletsByAddress, assets)

	mockUTXO := ledgerstate.NewUtxoDagMock(t, tangle.LedgerState.UTXODAG)
	tangle.LedgerState.UTXODAG = mockUTXO
	mockUTXO.On("InclusionState", assets.transactions["3"].ID()).Return(ledgerstate.Pending)
	mockUTXO.On("InclusionState", assets.transactions["0"].ID()).Return(ledgerstate.Pending)

	isEligibleFlag := runCheckEligibilityAndGetEligibility(t, tangle, assets.messages["1"].ID())
	assert.False(t, isEligibleFlag)

	isEligibleFlag = runCheckEligibilityAndGetEligibility(t, tangle, assets.messages["2"].ID())
	assert.False(t, isEligibleFlag)

	// reset mock, since calls can't be overridden
	mockUTXO.ExpectedCalls = make([]*mock.Call, 0)
	mockUTXO.On("InclusionState", assets.transactions["3"].ID()).Return(ledgerstate.Pending)
	mockUTXO.On("InclusionState", assets.transactions["0"].ID()).Return(ledgerstate.Confirmed)

	confirmedTransactionID := assets.transactions["0"].ID()

	err := tangle.EligibilityManager.updateEligibilityAfterDependencyConfirmation(&confirmedTransactionID)
	assert.NoError(t, err)

	tangle.Storage.MessageMetadata(assets.messages["1"].ID()).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.False(t, isEligibleFlag)
	tangle.Storage.MessageMetadata(assets.messages["2"].ID()).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.True(t, isEligibleFlag, "Message 1 isn't eligible")

	// reset mock, since calls can't be overridden
	mockUTXO.ExpectedCalls = make([]*mock.Call, 0)
	mockUTXO.On("InclusionState", assets.transactions["3"].ID()).Return(ledgerstate.Confirmed)
	mockUTXO.On("InclusionState", assets.transactions["0"].ID()).Return(ledgerstate.Confirmed)

	confirmedTransactionID = assets.transactions["3"].ID()

	err = tangle.EligibilityManager.updateEligibilityAfterDependencyConfirmation(&confirmedTransactionID)
	assert.NoError(t, err)

	tangle.Storage.MessageMetadata(assets.messages["1"].ID()).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.True(t, isEligibleFlag)
	tangle.Storage.MessageMetadata(assets.messages["2"].ID()).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	assert.True(t, isEligibleFlag)
}

type testAssets struct {
	messages     map[string]*Message
	transactions map[string]*ledgerstate.Transaction
	inputs       map[string]*ledgerstate.UTXOInput
	outputs      map[string]*ledgerstate.SigLockedSingleOutput
	outputsByID  map[ledgerstate.OutputID]ledgerstate.Output
}

func newTestAssets() testAssets {
	return testAssets{
		messages:     make(map[string]*Message),
		transactions: make(map[string]*ledgerstate.Transaction),
		inputs:       make(map[string]*ledgerstate.UTXOInput),
		outputs:      make(map[string]*ledgerstate.SigLockedSingleOutput),
		outputsByID:  make(map[ledgerstate.OutputID]ledgerstate.Output),
	}
}

func setupEligibilityTests(t *testing.T, tangle *Tangle) (map[string]wallet, map[ledgerstate.Address]wallet, *testAssets) {
	tangle.EligibilityManager.Setup()

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(10)
	wallets["GENESIS"] = w[0]
	wallets["A"] = w[1]
	wallets["B"] = w[2]
	wallets["C"] = w[3]
	wallets["D"] = w[4]
	wallets["E"] = w[5]
	wallets["F"] = w[6]
	wallets["H"] = w[7]
	wallets["I"] = w[8]
	wallets["J"] = w[9]
	for _, wlt := range wallets {
		walletsByAddress[wlt.address] = wlt
	}
	genesisBalance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 3,
		})
	genesisEssence := ledgerstate.NewTransactionEssence(
		0,
		time.Unix(DefaultGenesisTime, 0),
		identity.ID{},
		identity.ID{},
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))),
		ledgerstate.NewOutputs(ledgerstate.NewSigLockedColoredOutput(genesisBalance, wallets["GENESIS"].address)),
	)

	genesisTransaction := ledgerstate.NewTransaction(genesisEssence, ledgerstate.UnlockBlocks{ledgerstate.NewReferenceUnlockBlock(0)})
	stored, _, _ := tangle.LedgerState.UTXODAG.StoreTransaction(genesisTransaction)
	assert.True(t, stored, "genesis transaction stored")

	assets := newTestAssets()

	// Base message
	assets.inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(genesisTransaction.ID(), 0))
	assets.outputs["0A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	assets.outputs["0B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)
	assets.outputs["0C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	assets.transactions["0"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["GENESIS"]), ledgerstate.NewOutputs(assets.outputs["0A"], assets.outputs["0B"], assets.outputs["0C"]), assets.outputsByID, walletsByAddress, wallets["GENESIS"])
	assets.messages["0"] = newTestParentsPayloadMessage(assets.transactions["0"], []MessageID{EmptyMessageID}, []MessageID{})
	stored, _, _ = tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["0"])
	assert.True(t, stored)
	tangle.Storage.StoreMessage(assets.messages["0"])
	attachment, stored := tangle.Storage.StoreAttachment(assets.transactions["0"].ID(), assets.messages["0"].ID())
	assert.True(t, stored)
	attachment.Release()

	return wallets, walletsByAddress, &assets
}

func runCheckEligibilityAndGetEligibility(t *testing.T, tangle *Tangle, messageID MessageID) bool {
	err := tangle.EligibilityManager.checkEligibility(messageID)
	assert.NoError(t, err)
	var isEligibleFlag bool
	tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		isEligibleFlag = messageMetadata.IsEligible()
	})
	return isEligibleFlag
}

// create transaction 1 (msg 1) that takes input from tx 0 (msg 0) and does not approve msg 0 directly
func scenarioMessagesApproveEmptyID(t *testing.T, tangle *Tangle, wallets map[string]wallet, walletsByAddress map[ledgerstate.Address]wallet, assets *testAssets) {
	assets.inputs["1A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["A"])))
	assets.outputsByID[assets.inputs["1A"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0A"])[0]

	assets.outputs["1D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	assets.transactions["1"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["1A"]), ledgerstate.NewOutputs(assets.outputs["1D"]), assets.outputsByID, walletsByAddress)
	assets.messages["1"] = newTestParentsPayloadMessage(assets.transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})

	tangle.Storage.StoreMessage(assets.messages["1"])
	stored, _, _ := tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["1"])
	assert.True(t, stored)

	attachment, stored := tangle.Storage.StoreAttachment(assets.transactions["1"].ID(), assets.messages["1"].ID())
	assert.True(t, stored)
	attachment.Release()
}

// creates tx and msg 1 that directly approves msg 0 and uses tx0 outputs as its inputs
func scenarioMessagesApproveDependency(t *testing.T, tangle *Tangle, wallets map[string]wallet, walletsByAddress map[ledgerstate.Address]wallet, assets *testAssets) {
	assets.inputs["1A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["A"])))
	assets.outputsByID[assets.inputs["1A"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0A"])[0]
	assets.outputs["1D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)

	assets.transactions["1"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["1A"]), ledgerstate.NewOutputs(assets.outputs["1D"]), assets.outputsByID, walletsByAddress)
	assets.messages["1"] = newTestParentsPayloadMessage(assets.transactions["1"], []MessageID{assets.messages["0"].ID()}, []MessageID{})

	tangle.Storage.StoreMessage(assets.messages["1"])
	stored, _, _ := tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["1"])
	assert.True(t, stored)

	attachment, stored := tangle.Storage.StoreAttachment(assets.transactions["1"].ID(), assets.messages["1"].ID())
	assert.True(t, stored)
	attachment.Release()
}

// creates transaction that is dependent on two other transactions and is not connected by direct approval of their assets.messages
func scenarioMoreThanOneDependency(t *testing.T, tangle *Tangle, wallets map[string]wallet, walletsByAddress map[ledgerstate.Address]wallet, assets *testAssets) {
	// transaction 1
	assets.inputs["1A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["A"])))
	assets.outputsByID[assets.inputs["1A"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0A"])[0]
	assets.outputs["1D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)

	assets.transactions["1"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["1A"]), ledgerstate.NewOutputs(assets.outputs["1D"]), assets.outputsByID, walletsByAddress)
	assets.messages["1"] = newTestParentsPayloadMessage(assets.transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})

	// transaction 2
	assets.inputs["2B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["B"])))
	assets.outputsByID[assets.inputs["2B"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0B"])[0]
	assets.outputs["2E"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["E"].address)

	assets.transactions["2"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["2B"]), ledgerstate.NewOutputs(assets.outputs["2E"]), assets.outputsByID, walletsByAddress)
	assets.messages["2"] = newTestParentsPayloadMessage(assets.transactions["2"], []MessageID{EmptyMessageID}, []MessageID{})

	// transaction 3 dependent on transactions 1 and 2
	assets.inputs["3D"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["1"].ID(), selectIndex(assets.transactions["1"], wallets["D"])))
	assets.outputsByID[assets.inputs["3D"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["1D"])[0]
	assets.inputs["3E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["2"].ID(), selectIndex(assets.transactions["2"], wallets["E"])))
	assets.outputsByID[assets.inputs["3E"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["2E"])[0]
	assets.outputs["3F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)

	assets.transactions["3"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["3D"], assets.inputs["3E"]), ledgerstate.NewOutputs(assets.outputs["3F"]), assets.outputsByID, walletsByAddress)
	assets.messages["3"] = newTestParentsPayloadMessage(assets.transactions["3"], []MessageID{EmptyMessageID}, []MessageID{})

	// store all transactions and assets.messages
	tangle.Storage.StoreMessage(assets.messages["1"])
	stored, _, _ := tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["1"])
	assert.True(t, stored)
	attachment, stored := tangle.Storage.StoreAttachment(assets.transactions["1"].ID(), assets.messages["1"].ID())
	attachment.Release()
	assert.True(t, stored)
	tangle.Storage.StoreMessage(assets.messages["3"])
	stored, _, _ = tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["3"])
	assert.True(t, stored)
	attachment, stored = tangle.Storage.StoreAttachment(assets.transactions["3"].ID(), assets.messages["3"].ID())
	attachment.Release()
	assert.True(t, stored)
	tangle.Storage.StoreMessage(assets.messages["2"])
	stored, _, _ = tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["2"])
	assert.True(t, stored)
	attachment, stored = tangle.Storage.StoreAttachment(assets.transactions["2"].ID(), assets.messages["2"].ID())
	attachment.Release()
	assert.True(t, stored)
}

// two transactions 1 and 2 are dependent on the same transaction 0 and are not connected by direct approval between messages
// transaction is also dependent on other transaction 3 that will get confirmed before 0
func scenarioMoreThanOneDependentTransaction(t *testing.T, tangle *Tangle, wallets map[string]wallet, walletsByAddress map[ledgerstate.Address]wallet, assets *testAssets) {
	// transaction 3
	assets.inputs["3B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["B"])))
	assets.outputsByID[assets.inputs["3B"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0B"])[0]
	assets.outputs["3E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)

	assets.transactions["3"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["3B"]), ledgerstate.NewOutputs(assets.outputs["3E"]), assets.outputsByID, walletsByAddress)
	assets.messages["3"] = newTestParentsPayloadMessage(assets.transactions["3"], []MessageID{EmptyMessageID}, []MessageID{})

	// transaction 1
	assets.inputs["1A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["A"])))
	assets.outputsByID[assets.inputs["1A"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0A"])[0]
	assets.inputs["1E"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["3"].ID(), selectIndex(assets.transactions["3"], wallets["E"])))
	assets.outputsByID[assets.inputs["1E"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["3E"])[0]
	assets.outputs["1D"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["D"].address)

	assets.transactions["1"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["1A"], assets.inputs["1E"]), ledgerstate.NewOutputs(assets.outputs["1D"]), assets.outputsByID, walletsByAddress)
	assets.messages["1"] = newTestParentsPayloadMessage(assets.transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})

	// transaction 2
	assets.inputs["2C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(assets.transactions["0"].ID(), selectIndex(assets.transactions["0"], wallets["C"])))
	assets.outputsByID[assets.inputs["2C"].ReferencedOutputID()] = ledgerstate.NewOutputs(assets.outputs["0C"])[0]
	assets.outputs["2F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)

	assets.transactions["2"] = makeTransaction(ledgerstate.NewInputs(assets.inputs["2C"]), ledgerstate.NewOutputs(assets.outputs["2F"]), assets.outputsByID, walletsByAddress)
	assets.messages["2"] = newTestParentsPayloadMessage(assets.transactions["2"], []MessageID{EmptyMessageID}, []MessageID{})

	// store all transactions and messages
	tangle.Storage.StoreMessage(assets.messages["1"])
	stored, _, _ := tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["1"])
	assert.True(t, stored)
	attachment, stored := tangle.Storage.StoreAttachment(assets.transactions["1"].ID(), assets.messages["1"].ID())
	attachment.Release()
	assert.True(t, stored)

	tangle.Storage.StoreMessage(assets.messages["2"])
	stored, _, _ = tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["2"])
	assert.True(t, stored)
	attachment, stored = tangle.Storage.StoreAttachment(assets.transactions["2"].ID(), assets.messages["2"].ID())
	attachment.Release()
	assert.True(t, stored)

	tangle.Storage.StoreMessage(assets.messages["3"])
	stored, _, _ = tangle.LedgerState.UTXODAG.StoreTransaction(assets.transactions["3"])
	assert.True(t, stored)
	attachment, stored = tangle.Storage.StoreAttachment(assets.transactions["3"].ID(), assets.messages["3"].ID())
	attachment.Release()
	assert.True(t, stored)
}
