package newconflictdag

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

type Interface[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] interface {
	CreateConflict(id ConflictID, parentIDs *advancedset.AdvancedSet[ConflictID], resourceIDs *advancedset.AdvancedSet[ResourceID], initialWeight *weight.Weight) error
	ReadConsistent(callback func(conflictDAG ReadLockedConflictDAG[ConflictID, ResourceID, VotePower]) error) error
	JoinConflictSets(conflictID ConflictID, resourceIDs *advancedset.AdvancedSet[ResourceID]) error
	UpdateConflictParents(conflictID ConflictID, addedParentID ConflictID, removedParentIDs *advancedset.AdvancedSet[ConflictID]) error
	FutureCone(conflictIDs *advancedset.AdvancedSet[ConflictID]) (futureCone *advancedset.AdvancedSet[ConflictID])
	ConflictingConflicts(conflictID ConflictID) (conflictingConflicts *advancedset.AdvancedSet[ConflictID], exists bool)
	CastVotes(vote *vote.Vote[VotePower], conflictIDs *advancedset.AdvancedSet[ConflictID]) error
	AcceptanceState(conflictIDs *advancedset.AdvancedSet[ConflictID]) acceptance.State
	UnacceptedConflicts(conflictIDs *advancedset.AdvancedSet[ConflictID]) *advancedset.AdvancedSet[ConflictID]
	AllConflictsSupported(issuerID identity.ID, conflictIDs *advancedset.AdvancedSet[ConflictID]) bool
	EvictConflict(conflictID ConflictID) error

	ConflictSets(conflictID ConflictID) (conflictSetIDs *advancedset.AdvancedSet[ResourceID], exists bool)
	ConflictParents(conflictID ConflictID) (conflictIDs *advancedset.AdvancedSet[ConflictID], exists bool)
	ConflictSetMembers(conflictSetID ResourceID) (conflictIDs *advancedset.AdvancedSet[ConflictID], exists bool)
	ConflictWeight(conflictID ConflictID) int64
	ConflictChildren(conflictID ConflictID) (conflictIDs *advancedset.AdvancedSet[ConflictID], exists bool)
	ConflictVoters(conflictID ConflictID) (voters map[identity.ID]int64)
}

type ReadLockedConflictDAG[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] interface {
	LikedInstead(conflictIDs *advancedset.AdvancedSet[ConflictID]) *advancedset.AdvancedSet[ConflictID]
	FutureCone(conflictIDs *advancedset.AdvancedSet[ConflictID]) (futureCone *advancedset.AdvancedSet[ConflictID])
	ConflictingConflicts(conflictID ConflictID) (conflictingConflicts *advancedset.AdvancedSet[ConflictID], exists bool)
	AcceptanceState(conflictIDs *advancedset.AdvancedSet[ConflictID]) acceptance.State
	UnacceptedConflicts(conflictIDs *advancedset.AdvancedSet[ConflictID]) *advancedset.AdvancedSet[ConflictID]
}
