package branchdag

import (
	"github.com/iotaledger/hive.go/events"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Events struct {
	// BranchCreated gets triggered when a new Branch is created.
	BranchCreated *events.Event

	// BranchParentsUpdated gets triggered whenever a Branch's parents are updated.
	BranchParentsUpdated *events.Event
}

func NewEvents() *Events {
	return &Events{
		BranchCreated: events.NewEvent(func(handler interface{}, params ...interface{}) {
			handler.(func(BranchID))(params[0].(BranchID))
		}),
		BranchParentsUpdated: events.NewEvent(branchParentUpdateEventCaller),
	}
}

// BranchParentUpdate contains the new branch parents of a branch.
type BranchParentUpdate struct {
	ID         BranchID
	NewParents BranchIDs
}

func branchParentUpdateEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(branchParents *BranchParentUpdate))(params[0].(*BranchParentUpdate))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
