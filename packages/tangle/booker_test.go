package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenario_1(t *testing.T) {
	tangle := New(WithoutOpinionFormer(true))
	defer tangle.Shutdown()

	wallets := make(map[string]wallet)
	w := createWallets(9)
	wallets["GENESIS"] = w[0]
	wallets["A"] = w[1]
	wallets["B"] = w[2]
	wallets["C"] = w[3]
	wallets["D"] = w[4]
	wallets["E"] = w[5]
	wallets["F"] = w[6]
	wallets["H"] = w[7]
	wallets["I"] = w[8]

	snapshot := map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: {
			wallets["GENESIS"].address: ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: 3,
				})},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	messages := make(map[string]*Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	branches := make(map[string]ledgerstate.BranchID)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)

	branches["empty"] = ledgerstate.BranchID{}
	branches["green"] = ledgerstate.MasterBranchID
	branches["grey"] = ledgerstate.InvalidBranchID

	// Message 1
	inputs["GENESIS"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	outputs["A"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["A"].address)
	outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)
	outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)
	transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["GENESIS"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), wallets["GENESIS"])
	messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})
	tangle.Storage.StoreMessage(messages["1"])

	fmt.Println(messages["1"].ID())

	err := tangle.Booker.Book(messages["1"].ID())

	require.NoError(t, err)

	msgBranchID, err := messageBranchID(tangle, messages["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err := transactionBranchID(tangle, transactions["1"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 2
	inputs["B"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["B"])))
	inputs["C"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["C"])))
	outputs["E"] = ledgerstate.NewSigLockedSingleOutput(2, wallets["E"].address)
	transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["B"], inputs["C"]), ledgerstate.NewOutputs(outputs["E"]), sortWallets(transactions["1"].Essence().Outputs(), []wallet{wallets["B"], wallets["C"]})...)
	messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["2"])

	err = tangle.Booker.Book(messages["2"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 3 (Reattachemnt of transaction 2)
	messages["3"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["3"])

	err = tangle.Booker.Book(messages["3"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["2"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 4
	inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
	outputs["D"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["D"].address)
	transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["D"]), wallets["A"])
	messages["4"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["4"])

	err = tangle.Booker.Book(messages["4"].ID())
	require.NoError(t, err)

	msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["green"], txBranchID)

	// Message 5
	outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)
	transactions["4"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["F"]), wallets["A"])
	messages["5"] = newTestParentsPayloadMessage(transactions["4"], []MessageID{EmptyMessageID, messages["1"].ID()}, []MessageID{})
	tangle.Storage.StoreMessage(messages["5"])

	err = tangle.Booker.Book(messages["5"].ID())
	require.NoError(t, err)

	branches["yellow"] = ledgerstate.NewBranchID(transactions["4"].ID())

	msgBranchID, err = messageBranchID(tangle, messages["5"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["4"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["yellow"], txBranchID)

	branches["red"] = ledgerstate.NewBranchID(transactions["3"].ID())

	// msgBranchID, err = messageBranchID(tangle, messages["4"].ID())
	// require.NoError(t, err)
	// assert.Equal(t, branches["red"], msgBranchID)

	txBranchID, err = transactionBranchID(tangle, transactions["3"].ID())
	require.NoError(t, err)
	assert.Equal(t, branches["red"], txBranchID)
}

func TestMarkerIndexBranchMapping_String(t *testing.T) {
	mapping := NewMarkerIndexBranchIDMapping(1337)
	mapping.SetBranchID(4, ledgerstate.UndefinedBranchID)
	mapping.SetBranchID(24, ledgerstate.MasterBranchID)

	fmt.Println(mapping)

	mappingClone, _, err := MarkerIndexBranchIDMappingFromBytes(mapping.Bytes())
	require.NoError(t, err)

	fmt.Println(mappingClone)
}

func TestBooker_allTransactionsApprovedByMessage(t *testing.T) {
	// TODO:
}

func TestBooker_transactionApprovedByMessage(t *testing.T) {
	_tangle := New()
	_tangle.Booker = NewBooker(_tangle)
	defer _tangle.Shutdown()

	sharedMarkers := markers.NewMarkers()
	sharedMarkers.Set(1, 1)

	txA := randomTransaction()
	// direct approval
	msgA := newTestDataMessage("a")
	_tangle.Storage.StoreMessage(msgA)
	_tangle.Storage.MessageMetadata(msgA.ID()).Consume(func(messageMetadata *MessageMetadata) {
		futureMarkers := markers.NewMarkers()
		futureMarkers.Set(2, 3)
		messageMetadata.SetStructureDetails(&markers.StructureDetails{
			Rank:          0,
			IsPastMarker:  true,
			PastMarkers:   sharedMarkers,
			FutureMarkers: futureMarkers,
		})
	})
	cachedAttachment, stored := _tangle.Storage.StoreAttachment(txA.ID(), msgA.ID())
	assert.True(t, stored)
	cachedAttachment.Release()
	assert.True(t, _tangle.Booker.transactionApprovedByMessage(txA.ID(), msgA.ID()))

	// indirect approval
	msgB := newTestParentsDataMessage("b", []MessageID{msgA.ID()}, []MessageID{EmptyMessageID})
	_tangle.Storage.StoreMessage(msgB)
	_tangle.Storage.MessageMetadata(msgB.ID()).Consume(func(messageMetadata *MessageMetadata) {
		futureMarkers := markers.NewMarkers()
		futureMarkers.Set(2, 3)
		messageMetadata.SetStructureDetails(&markers.StructureDetails{
			Rank:          1,
			IsPastMarker:  false,
			PastMarkers:   sharedMarkers,
			FutureMarkers: futureMarkers,
		})
	})
	assert.True(t, _tangle.Booker.transactionApprovedByMessage(txA.ID(), msgB.ID()))
}

func TestBooker_branchIDsOfStrongParents(t *testing.T) {
	_tangle := New()
	_tangle.Booker = NewBooker(_tangle)
	defer _tangle.Shutdown()

	var strongParentBranchIDs []ledgerstate.BranchID
	var strongParents []MessageID
	nStrongParents := 2

	for i := 0; i < nStrongParents; i++ {
		parent := newTestDataMessage(fmt.Sprint(i))
		branchID := randomBranchID()
		_tangle.Storage.StoreMessage(parent)
		_tangle.Storage.MessageMetadata(parent.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchID)
		})
		strongParentBranchIDs = append(strongParentBranchIDs, branchID)
		strongParents = append(strongParents, parent.ID())
	}

	weakParent := newTestDataMessage("weak")
	_tangle.Storage.StoreMessage(weakParent)
	_tangle.Storage.MessageMetadata(weakParent.ID()).Consume(func(messageMetadata *MessageMetadata) {
		messageMetadata.SetBranchID(randomBranchID())
	})

	msg := newTestParentsDataMessage("msg", strongParents, []MessageID{weakParent.ID()})
	_tangle.Storage.StoreMessage(msg)
	branchIDs := _tangle.Booker.branchIDsOfStrongParents(msg)

	assert.Equal(t, nStrongParents, len(branchIDs.Slice()))
	for _, branchID := range strongParentBranchIDs {
		assert.True(t, branchIDs.Contains(branchID))
	}
}

func randomBranchID() ledgerstate.BranchID {
	bytes := randomBytes(ledgerstate.BranchIDLength)
	result, _, _ := ledgerstate.BranchIDFromBytes(bytes)
	return result
}

func randomTransaction() *ledgerstate.Transaction {
	ID, _ := identity.RandomID()
	var inputs ledgerstate.Inputs
	var outputs ledgerstate.Outputs
	essence := ledgerstate.NewTransactionEssence(1, time.Now(), ID, ID, inputs, outputs)
	var unlockBlocks ledgerstate.UnlockBlocks
	return ledgerstate.NewTransaction(essence, unlockBlocks)
}

func messageBranchID(tangle *Tangle, messageID MessageID) (branchID ledgerstate.BranchID, err error) {
	if !tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		branchID = messageMetadata.BranchID()
		fmt.Println(messageID)
		fmt.Println(messageMetadata.StructureDetails())
	}) {
		return branchID, fmt.Errorf("missing message metadata")
	}
	return
}

func transactionBranchID(tangle *Tangle, transactionID ledgerstate.TransactionID) (branchID ledgerstate.BranchID, err error) {
	if !tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(metadata *ledgerstate.TransactionMetadata) {
		branchID = metadata.BranchID()
	}) {
		return branchID, fmt.Errorf("missing transaction metadata")
	}
	return
}

func makeTransaction(inputs ledgerstate.Inputs, outputs ledgerstate.Outputs, wallets ...wallet) *ledgerstate.Transaction {
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(wallets[i].sign(txEssence))
	}
	return ledgerstate.NewTransaction(txEssence, unlockBlocks)
}

func selectIndex(transaction *ledgerstate.Transaction, w wallet) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if w.address == output.(*ledgerstate.SigLockedSingleOutput).Address() {
			return uint16(i)
		}
	}
	return
}

func sortWallets(outputs ledgerstate.Outputs, w []wallet) (wallets []wallet) {
	for _, output := range outputs {
		for _, wallet := range w {
			if wallet.address == output.(*ledgerstate.SigLockedSingleOutput).Address() {
				wallets = append(wallets, wallet)
			}
		}
	}
	return
}
