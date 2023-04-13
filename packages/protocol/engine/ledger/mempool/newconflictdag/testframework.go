package newconflictdag

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestFramework struct {
	test        *testing.T
	ConflictDAG *ConflictDAG[TestID, TestID, votes.MockedVotePower]
	Weights     *sybilprotection.Weights

	conflictsByAlias    map[string]*Conflict[TestID, TestID, votes.MockedVotePower]
	conflictSetsByAlias map[string]*ConflictSet[TestID, TestID, votes.MockedVotePower]
}

// NewTestFramework creates a new instance of the TestFramework with one default output "Genesis" which has to be
// consumed by the first transaction.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) *TestFramework {
	return options.Apply(&TestFramework{
		test:                test,
		conflictsByAlias:    make(map[string]*Conflict[TestID, TestID, votes.MockedVotePower]),
		conflictSetsByAlias: make(map[string]*ConflictSet[TestID, TestID, votes.MockedVotePower]),
	}, opts, func(t *TestFramework) {
		if t.Weights == nil {
			t.Weights = sybilprotection.NewWeights(mapdb.NewMapDB())
		}

		if t.ConflictDAG == nil {
			t.ConflictDAG = New[TestID, TestID, votes.MockedVotePower](acceptance.ThresholdProvider(t.Weights.TotalWeight))
		}
	})
}

func (t *TestFramework) CreateConflict(alias string, parentIDs []string, resourceAliases []string, initialWeight *weight.Weight) (*Conflict[TestID, TestID, votes.MockedVotePower], error) {
	if err := t.ConflictDAG.CreateConflict(NewTestID(alias), t.ConflictIDs(parentIDs...), t.ConflictSetIDs(resourceAliases...), initialWeight); err != nil {
		return nil, err
	}

	t.conflictsByAlias[alias] = lo.Return1(t.ConflictDAG.conflictsByID.Get(NewTestID(alias)))

	for _, resourceAlias := range resourceAliases {
		t.conflictSetsByAlias[resourceAlias] = lo.Return1(t.ConflictDAG.conflictSetsByID.Get(NewTestID(resourceAlias)))
	}

	return t.conflictsByAlias[alias], nil
}

func (t *TestFramework) ConflictIDs(aliases ...string) (conflictIDs []TestID) {
	for _, alias := range aliases {
		conflictIDs = append(conflictIDs, NewTestID(alias))
	}

	return conflictIDs
}

func (t *TestFramework) ConflictSetIDs(aliases ...string) (conflictSetIDs []TestID) {
	for _, alias := range aliases {
		conflictSetIDs = append(conflictSetIDs, NewTestID(alias))
	}

	return conflictSetIDs
}

func (t *TestFramework) Conflict(alias string) *Conflict[TestID, TestID, votes.MockedVotePower] {
	conflict, ok := t.conflictsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Conflict alias %s not registered", alias))
	}

	return conflict
}

func (t *TestFramework) ConflictSet(alias string) *ConflictSet[TestID, TestID, votes.MockedVotePower] {
	conflictSet, ok := t.conflictSetsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("ConflictSet alias %s not registered", alias))
	}

	return conflictSet
}

func (t *TestFramework) Weight() *weight.Weight {
	return weight.New(t.Weights)
}

func (t *TestFramework) UpdateConflictParents(conflictAlias string, addedParentID string, removedParentIDs ...string) bool {
	return t.ConflictDAG.UpdateConflictParents(NewTestID(conflictAlias), NewTestID(addedParentID), t.ConflictIDs(removedParentIDs...)...)
}

func (t *TestFramework) JoinConflictSets(conflictAlias string, resourceAliases ...string) []*ConflictSet[TestID, TestID, votes.MockedVotePower] {
	return lo.Values(t.ConflictDAG.JoinConflictSets(NewTestID(conflictAlias), t.ConflictSetIDs(resourceAliases...)...))
}

func (t *TestFramework) LikedInstead(conflictAliases ...string) []*Conflict[TestID, TestID, votes.MockedVotePower] {
	return lo.Values(t.ConflictDAG.LikedInstead(t.ConflictIDs(conflictAliases...)...))
}

func (t *TestFramework) CastVotes(vote *vote.Vote[votes.MockedVotePower], conflictAliases ...string) error {
	return t.ConflictDAG.CastVotes(vote, t.ConflictIDs(conflictAliases...)...)
}

func WithWeights(weights *sybilprotection.Weights) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.Weights = weights
	}
}

type TestID struct {
	utxo.TransactionID
}

func NewTestID(alias string) TestID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	testID := utxo.NewTransactionID(hashedAlias[:])
	testID.RegisterAlias(alias)

	return TestID{testID}
}

func (id TestID) String() string {
	return strings.Replace(id.TransactionID.String(), "TransactionID", "TestID", 1)
}
