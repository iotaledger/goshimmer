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
