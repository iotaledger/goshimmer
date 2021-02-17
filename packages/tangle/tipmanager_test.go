package tangle

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTipManager_AddTip(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager

	// not eligible messages -> nothing is added
	{
		message := newTestParentsDataMessage("testmessage", []MessageID{EmptyMessageID}, []MessageID{})
		tangle.Storage.StoreMessage(message)
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetEligible(false)
		})

		tipManager.AddTip(message)
		assert.Equal(t, 0, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// TODO: payload not liked -> nothing is added
	{
	}

	// clean up
	err := tangle.Prune()
	require.NoError(t, err)

	// set up scenario (images/tipmanager-add-tips.png)
	messages := make(map[string]*Message)

	// Message 1
	{
		messages["1"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["1"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID())
	}

	// Message 2
	{
		messages["2"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID(), messages["2"].ID())
	}

	// Message 3
	{
		messages["3"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
	}

	// Message 4
	{
		messages["4"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["2"].ID()}, []MessageID{messages["3"].ID()})
		tipManager.AddTip(messages["4"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 1, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID())
	}

	// Message 5
	{
		messages["5"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["3"].ID(), messages["4"].ID()}, []MessageID{})
		tipManager.AddTip(messages["5"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 2, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID(), messages["5"].ID())
	}

	// Message 6
	{
		messages["6"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["3"].ID()}, []MessageID{messages["4"].ID(), messages["5"].ID()})
		tipManager.AddTip(messages["6"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["6"].ID())
	}
}

func TestTipManager_DataMessageTips(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager

	// set up scenario (images/tipmanager-DataMessageTips-test.png)
	messages := make(map[string]*Message)

	// without any tip -> genesis
	{
		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, EmptyMessageID)
		assert.Empty(t, weakParents)
	}

	// without any count -> 1 tip, in this case genesis
	{
		strongParents, weakParents := tipManager.Tips(nil, 0, 0)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, EmptyMessageID)
		assert.Empty(t, weakParents)
	}

	// Message 1
	{
		messages["1"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["1"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID())

		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["1"].ID())
		assert.Empty(t, weakParents)
	}

	// Message 2
	{
		messages["2"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{EmptyMessageID}, []MessageID{})
		tipManager.AddTip(messages["2"])

		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["1"].ID(), messages["2"].ID())

		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 2)
		assert.Contains(t, strongParents, messages["1"].ID(), messages["2"].ID())
		assert.Empty(t, weakParents)
	}

	// Message 3
	{
		messages["3"] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID(), messages["2"].ID()}, []MessageID{})
		tipManager.AddTip(messages["3"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())

		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["3"].ID())
		assert.Empty(t, weakParents)
	}

	// Message 4
	{
		messages["4"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["2"].ID()}, []MessageID{messages["3"].ID()})
		tipManager.AddTip(messages["4"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 1, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID())

		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["3"].ID())
		assert.Len(t, weakParents, 1)
		assert.Contains(t, weakParents, messages["4"].ID())
	}

	// Message 5
	{
		messages["5"] = createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle, []MessageID{messages["3"].ID(), messages["4"].ID()}, []MessageID{})
		tipManager.AddTip(messages["5"])

		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 2, tipManager.WeakTipCount())
		assert.Contains(t, tipManager.strongTips.Keys(), messages["3"].ID())
		assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID(), messages["5"].ID())

		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 1)
		assert.Contains(t, strongParents, messages["3"].ID())
		assert.Len(t, weakParents, 2)
		assert.Contains(t, weakParents, messages["4"].ID(), messages["5"].ID())
	}

	// Add Message 6-12
	{
		strongTips := make([]MessageID, 0, 9)
		strongTips = append(strongTips, messages["3"].ID())
		for count, n := range []int{6, 7, 8, 9, 10, 11, 12, 13} {
			nString := strconv.Itoa(n)
			messages[nString] = createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle, []MessageID{messages["1"].ID()}, []MessageID{})
			tipManager.AddTip(messages[nString])
			strongTips = append(strongTips, messages[nString].ID())

			assert.Equalf(t, count+2, tipManager.StrongTipCount(), "StrongTipCount does not match after adding Message %d", n)
			assert.Equalf(t, 2, tipManager.WeakTipCount(), "WeakTipCount does not match after adding Message %d", n)
			assert.ElementsMatchf(t, tipManager.strongTips.Keys(), strongTips, "Elements in strongTips do not match after adding Message %d", n)
			assert.Contains(t, tipManager.weakTips.Keys(), messages["4"].ID(), messages["5"].ID())
		}
	}

	// now we have strongTips: 9, weakTips: 2
	// Tips(2,2) -> 2,2
	{
		strongParents, weakParents := tipManager.Tips(nil, 2, 2)
		assert.Len(t, strongParents, 2)
		assert.Len(t, weakParents, 2)
	}
	// Tips(8,2) -> 8,0
	{
		strongParents, weakParents := tipManager.Tips(nil, 8, 2)
		assert.Len(t, strongParents, 8)
		assert.Len(t, weakParents, 0)
	}
	// Tips(9,2) -> 8,0
	{
		strongParents, weakParents := tipManager.Tips(nil, 9, 2)
		assert.Len(t, strongParents, 8)
		assert.Len(t, weakParents, 0)
	}
	// Tips(7,2) -> 7,1
	{
		strongParents, weakParents := tipManager.Tips(nil, 7, 1)
		assert.Len(t, strongParents, 7)
		assert.Len(t, weakParents, 1)
	}
	// Tips(6,2) -> 6,2
	{
		strongParents, weakParents := tipManager.Tips(nil, 6, 2)
		assert.Len(t, strongParents, 6)
		assert.Len(t, weakParents, 2)
	}
	// Tips(4,1) -> 4,1
	{
		strongParents, weakParents := tipManager.Tips(nil, 4, 1)
		assert.Len(t, strongParents, 4)
		assert.Len(t, weakParents, 1)
	}
	// Tips(0,2) -> 1,2
	{
		strongParents, weakParents := tipManager.Tips(nil, 0, 2)
		assert.Len(t, strongParents, 1)
		assert.Len(t, weakParents, 2)
	}
}

func TestTipManager_TransactionTips(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	tipManager := tangle.TipManager

	wallets := make(map[string]wallet)
	walletsByAddress := make(map[ledgerstate.Address]wallet)
	w := createWallets(26)
	wallets["G1"] = w[0]
	wallets["G2"] = w[1]
	wallets["A"] = w[2]
	wallets["B"] = w[3]
	wallets["C"] = w[4]
	wallets["D"] = w[5]
	wallets["E"] = w[6]
	wallets["F"] = w[7]
	wallets["H"] = w[8]
	wallets["I"] = w[9]
	wallets["J"] = w[10]
	wallets["K"] = w[11]
	wallets["L"] = w[12]
	wallets["M"] = w[13]
	wallets["N"] = w[14]
	wallets["O"] = w[15]
	wallets["P"] = w[16]
	wallets["Q"] = w[17]
	wallets["R"] = w[18]
	wallets["S"] = w[19]
	wallets["T"] = w[20]
	wallets["U"] = w[21]
	wallets["V"] = w[22]
	wallets["X"] = w[23]
	wallets["Y"] = w[24]
	wallets["Z"] = w[25]

	for _, wallet := range wallets {
		walletsByAddress[wallet.address] = wallet
	}

	g1Balance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 5,
		})
	g2Balance := ledgerstate.NewColoredBalances(
		map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: 8,
		})
	snapshot := map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: {
			wallets["G1"].address: g1Balance,
			wallets["G2"].address: g2Balance,
		},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	messages := make(map[string]*Message)
	transactions := make(map[string]*ledgerstate.Transaction)
	inputs := make(map[string]*ledgerstate.UTXOInput)
	outputs := make(map[string]*ledgerstate.SigLockedSingleOutput)
	outputsByID := make(map[ledgerstate.OutputID]ledgerstate.Output)

	// mock the Tangle's PayloadOpinionProvider so that we can add transaction payloads without actually building opinions
	mockOpinionProvider := &mockPayloadOpinionProvider{
		payloadOpinionFunc: func(messageID MessageID) bool {
			for _, msg := range messages {
				if msg.ID() == messageID {
					return true
				}
			}
			return false
		},
	}
	tangle.PayloadOpinionProvider = mockOpinionProvider
	tangle.OpinionFormer.payloadOpinionProvider = mockOpinionProvider

	// Message 1
	{
		inputs["G1"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
		outputs["A"] = ledgerstate.NewSigLockedSingleOutput(3, wallets["A"].address)
		outputs["B"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["B"].address)
		outputs["C"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["C"].address)

		transactions["1"] = makeTransaction(ledgerstate.NewInputs(inputs["G1"]), ledgerstate.NewOutputs(outputs["A"], outputs["B"], outputs["C"]), outputsByID, walletsByAddress, wallets["G1"])
		messages["1"] = newTestParentsPayloadMessage(transactions["1"], []MessageID{EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["1"])

		tipManager.AddTip(messages["1"])
		assert.Equal(t, 1, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 2
	{
		inputs["G2"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 1))
		outputs["D"] = ledgerstate.NewSigLockedSingleOutput(6, wallets["D"].address)
		outputs["E"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["E"].address)
		outputs["F"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["F"].address)

		transactions["2"] = makeTransaction(ledgerstate.NewInputs(inputs["G2"]), ledgerstate.NewOutputs(outputs["D"], outputs["E"], outputs["F"]), outputsByID, walletsByAddress, wallets["G2"])
		messages["2"] = newTestParentsPayloadMessage(transactions["2"], []MessageID{EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["2"])

		tipManager.AddTip(messages["2"])
		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

	// Message 3
	{
		inputs["A"] = ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(transactions["1"].ID(), selectIndex(transactions["1"], wallets["A"])))
		outputsByID[inputs["A"].ReferencedOutputID()] = ledgerstate.NewOutputs(outputs["A"])[0]
		outputs["H"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["H"].address)
		outputs["I"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["I"].address)
		outputs["J"] = ledgerstate.NewSigLockedSingleOutput(1, wallets["J"].address)

		transactions["3"] = makeTransaction(ledgerstate.NewInputs(inputs["A"]), ledgerstate.NewOutputs(outputs["H"], outputs["I"], outputs["J"]), outputsByID, walletsByAddress)
		messages["3"] = newTestParentsPayloadMessage(transactions["3"], []MessageID{messages["1"].ID(), EmptyMessageID}, []MessageID{})

		storeBookLikeMessage(t, tangle, messages["3"])

		tipManager.AddTip(messages["3"])
		assert.Equal(t, 2, tipManager.StrongTipCount())
		assert.Equal(t, 0, tipManager.WeakTipCount())
	}

}

func storeBookLikeMessage(t *testing.T, tangle *Tangle, message *Message) {
	// we need to store and book transactions so that we also have attachments of transactions available
	tangle.Storage.StoreMessage(message)
	// TODO: CheckTransaction should be removed here once the booker passes on errors
	//_, err := tangle.LedgerState.utxoDAG.CheckTransaction(message.payload.(*ledgerstate.Transaction))
	//require.NoError(t, err)
	fmt.Println("Booking", message.ID())
	err := tangle.Booker.Book(message.ID())
	require.NoError(t, err)

	tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		// make sure that everything was booked into master branch
		require.True(t, messageMetadata.booked)
		require.Equal(t, ledgerstate.MasterBranchID, messageMetadata.BranchID())

		messageMetadata.SetEligible(true)

		//fmt.Println(messageMetadata)
	})
}

func createAndStoreEligibleTestParentsDataMessageInMasterBranch(tangle *Tangle, strongParents, weakParents MessageIDs) (message *Message) {
	message = newTestParentsDataMessage("testmessage", strongParents, weakParents)
	tangle.Storage.StoreMessage(message)
	message.setMessageMetadata(tangle, true, ledgerstate.MasterBranchID)

	return
}

func createAndStoreEligibleTestParentsDataMessageInInvalidBranch(tangle *Tangle, strongParents, weakParents MessageIDs) (message *Message) {
	message = newTestParentsDataMessage("testmessage", strongParents, weakParents)
	tangle.Storage.StoreMessage(message)
	message.setMessageMetadata(tangle, true, ledgerstate.InvalidBranchID)

	return
}

func (m *Message) setMessageMetadata(tangle *Tangle, eligible bool, branchID ledgerstate.BranchID) {
	tangle.Storage.MessageMetadata(m.ID()).Consume(func(messageMetadata *MessageMetadata) {
		messageMetadata.SetEligible(eligible)
		messageMetadata.SetBranchID(branchID)
	})
}

type mockPayloadOpinionProvider struct {
	payloadOpinionFunc func(messageID MessageID) bool
}

func (m *mockPayloadOpinionProvider) Evaluate(id MessageID) {
	panic("implement me")
}

func (m *mockPayloadOpinionProvider) Opinion(id MessageID) bool {
	return m.payloadOpinionFunc(id)
}

func (m *mockPayloadOpinionProvider) Setup(event *events.Event) {
	panic("implement me")
}

func (m *mockPayloadOpinionProvider) Shutdown() {
}

func (m *mockPayloadOpinionProvider) Vote() *events.Event {
	panic("implement me")
}
func (m *mockPayloadOpinionProvider) VoteError() *events.Event {
	panic("implement me")
}
func (m *mockPayloadOpinionProvider) ProcessVote(*vote.OpinionEvent) {
	panic("implement me")
}
