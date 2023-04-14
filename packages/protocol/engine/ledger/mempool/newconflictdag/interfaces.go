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
	CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID, initialWeight *weight.Weight) error
	Read(callback func(conflictDAG ReadLockedConflictDAG[ConflictID, ResourceID, VotePower]) error) error
	JoinConflictSets(conflictID ConflictID, resourceIDs ...ResourceID) error
	UpdateConflictParents(conflictID ConflictID, addedParentID ConflictID, removedParentIDs ...ConflictID) error
	FutureCone(conflictIDs *advancedset.AdvancedSet[ConflictID]) (futureCone *advancedset.AdvancedSet[ConflictID])
	ConflictingConflicts(conflictID ConflictID) (conflictingConflicts *advancedset.AdvancedSet[ConflictID], exists bool)
	CastVotes(vote *vote.Vote[VotePower], conflictIDs ...ConflictID) error
	AcceptanceState(conflictIDs ...ConflictID) acceptance.State
	UnacceptedConflicts(conflictIDs ...ConflictID) *advancedset.AdvancedSet[ConflictID]
	AllConflictsSupported(issuerID identity.ID, conflictIDs ...ConflictID) bool
	EvictConflict(conflictID ConflictID) error
}

type ReadLockedConflictDAG[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] interface {
	LikedInstead(conflictIDs ...ConflictID) *advancedset.AdvancedSet[ConflictID]
	FutureCone(conflictIDs *advancedset.AdvancedSet[ConflictID]) (futureCone *advancedset.AdvancedSet[ConflictID])
	ConflictingConflicts(conflictID ConflictID) (conflictingConflicts *advancedset.AdvancedSet[ConflictID], exists bool)
	AcceptanceState(conflictIDs ...ConflictID) acceptance.State
	UnacceptedConflicts(conflictIDs ...ConflictID) *advancedset.AdvancedSet[ConflictID]
}
