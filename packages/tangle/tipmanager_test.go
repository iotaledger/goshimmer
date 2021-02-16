package tangle

import (
	"strconv"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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
