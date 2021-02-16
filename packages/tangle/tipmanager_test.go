package tangle

import (
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
