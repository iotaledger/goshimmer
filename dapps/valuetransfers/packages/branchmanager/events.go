package branchmanager

import (
	"github.com/iotaledger/hive.go/events"
)

// Events is a container for the different kind of events of the BranchManager.
type Events struct {
	// BranchPreferred gets triggered whenever a Branch becomes preferred that was not preferred before.
	BranchPreferred *events.Event

	// BranchUnpreferred gets triggered whenever a Branch becomes unpreferred that was preferred before.
	BranchUnpreferred *events.Event

	// BranchLiked gets triggered whenever a Branch becomes liked that was not liked before.
	BranchLiked *events.Event

	// BranchLiked gets triggered whenever a Branch becomes preferred that was not preferred before.
	BranchDisliked *events.Event

	// BranchFinalized gets triggered when a decision on a Branch is finalized and there will be no further state
	// changes regarding its preferred state.
	BranchFinalized *events.Event

	// BranchConfirmed gets triggered whenever a Branch becomes confirmed that was not confirmed before.
	BranchConfirmed *events.Event

	// BranchRejected gets triggered whenever a Branch becomes rejected that was not rejected before.
	BranchRejected *events.Event
}

func branchCaller(handler interface{}, params ...interface{}) {
	handler.(func(branch *CachedBranch))(params[0].(*CachedBranch).Retain())
}
