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

func TestConflictDAG(t *testing.T) {
	tests.Run(t, newTestFramework)
}

func newTestFramework(t *testing.T) *tests.TestFramework[utxo.TransactionID, utxo.OutputID, vote.MockedPower] {
	weights := sybilprotection.NewWeights(mapdb.NewMapDB())
	conflictDAG := New[utxo.TransactionID, utxo.OutputID, vote.MockedPower](weights.NewWeightedSet())

	return tests.NewTestFramework[utxo.TransactionID, utxo.OutputID, vote.MockedPower](t, conflictDAG, transactionID, outputID)
}

func transactionID(alias string) utxo.TransactionID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	var result utxo.TransactionID
	_ = lo.PanicOnErr(result.FromBytes(hashedAlias[:]))

	return result
}

func outputID(alias string) utxo.OutputID {
	hashedAlias := blake2b.Sum256([]byte(alias))

	var result utxo.OutputID
	_ = lo.PanicOnErr(result.TransactionID.FromBytes(hashedAlias[:]))

	return result
}
