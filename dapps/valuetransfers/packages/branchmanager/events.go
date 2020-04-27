package branchmanager

import (
	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	BranchPreferred   *events.Event
	BranchUnpreferred *events.Event
	BranchLiked       *events.Event
	BranchDisliked    *events.Event
}
