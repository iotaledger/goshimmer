package conflictdagv1

import (
	"testing"

	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag/tests"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
)

// TestConflictDAG runs the generic tests for the ConflictDAG.
func TestConflictDAG(t *testing.T) {
	tests.TestAll(t, newTestFramework)
}

// newTestFramework creates a new instance of the TestFramework for internal unit tests.
func newTestFramework(t *testing.T) *tests.Framework[utxo.TransactionID, utxo.OutputID, vote.MockedPower] {
	validators := sybilprotection.NewWeights(mapdb.NewMapDB()).NewWeightedSet()

	return tests.NewFramework[utxo.TransactionID, utxo.OutputID, vote.MockedPower](
		t,
		New[utxo.TransactionID, utxo.OutputID, vote.MockedPower](validators),
		sybilprotection.NewWeightedSetTestFramework(t, validators),
		transactionID,
		outputID,
	)
}

// transactionID creates a (made up) TransactionID from the given alias.
func transactionID(alias string) utxo.TransactionID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	var result utxo.TransactionID
	_ = lo.PanicOnErr(result.FromBytes(hashedAlias[:]))

	return result
}

// outputID creates a (made up) OutputID from the given alias.
func outputID(alias string) utxo.OutputID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	var result utxo.OutputID
	_ = lo.PanicOnErr(result.TransactionID.FromBytes(hashedAlias[:]))

	return result
}
