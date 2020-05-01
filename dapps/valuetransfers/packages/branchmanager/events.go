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
}
