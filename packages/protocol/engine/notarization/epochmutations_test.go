package notarization

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestMutationFactory(t *testing.T) {
	tf := NewTestFramework(t)

	// create transactions
	tf.CreateTransaction("tx1.1", 1)
	tf.CreateTransaction("tx2.1", 2)
	tf.CreateTransaction("tx2.2", 2)
	tf.CreateTransaction("tx3.1", 3)

	// create issuers
	tf.CreateIssuer("Batman", 10)
	tf.CreateIssuer("Robin", 10)
	tf.CreateIssuer("Joker", 10)
	tf.CreateIssuer("Superman", 10)

	// create blocks
	tf.CreateBlock("1.1", 1)
	tf.CreateBlock("2.1", 2, models.WithIssuer(tf.Issuer("Batman")))
	tf.CreateBlock("2.2", 2, models.WithIssuer(tf.Issuer("Robin")))
	tf.CreateBlock("2.3", 2, models.WithIssuer(tf.Issuer("Joker")))
	tf.CreateBlock("3.1", 3, models.WithIssuer(tf.Issuer("Batman")))
	tf.CreateBlock("3.2", 3, models.WithIssuer(tf.Issuer("Superman")))

	// commit epoch 1 (empty)
	tf.AssertCommit(1, nil, nil, nil, 0)

	// mutate epoch 1 (errors expected)
	require.Error(t, tf.AddAcceptedBlock("1.1"))
	require.Error(t, tf.RemoveAcceptedBlock("1.1"))
	require.NoError(t, tf.UpdateTransactionInclusion("tx1.1", 3, 5))
	require.Error(t, tf.UpdateTransactionInclusion("tx3.1", 2, 1))
	tf.AssertCommit(1, nil, nil, nil, 0, true)

	// mutate epoch 2
	require.NoError(t, tf.AddAcceptedBlock("2.1"))
	require.NoError(t, tf.AddAcceptedBlock("2.2"))
	require.NoError(t, tf.AddAcceptedBlock("2.3"))
	require.NoError(t, tf.AddAcceptedTransaction("tx2.1"))
	require.NoError(t, tf.AddAcceptedTransaction("tx2.2"))
	require.NoError(t, tf.RemoveAcceptedTransaction("tx2.2"))

	// mutate epoch 3
	require.NoError(t, tf.AddAcceptedBlock("3.1"))
	require.NoError(t, tf.AddAcceptedBlock("3.2"))
	require.NoError(t, tf.AddAcceptedTransaction("tx3.1"))
	require.NoError(t, tf.UpdateTransactionInclusion("tx3.1", 3, 2))
	require.NoError(t, tf.RemoveAcceptedBlock("3.2"))

	// assert commitment of epoch 2
	tf.AssertCommit(2, []string{"2.1", "2.2", "2.3"}, []string{"tx2.1", "tx3.1"}, []string{"Batman", "Robin", "Joker"}, 30, false)

	// assert commitment of epoch 3
	// The CW is 10 because the epoch mutation does not accumulate the weights, the notarization manager does.
	tf.AssertCommit(3, []string{"3.1"}, []string{}, []string{"Batman"}, 10, false)
}

func TestMutationFactory_AddAcceptedBlock(t *testing.T) {
	mutationFactory := NewEpochMutations(sybilprotection.NewWeights(mapdb.NewMapDB()), 2)

	block := models.NewBlock(
		models.WithIssuingTime(epoch.Index(3).EndTime()),
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
	)
	require.NoError(t, block.DetermineID())

	require.NoError(t, mutationFactory.AddAcceptedBlock(block))
	require.True(t, mutationFactory.acceptedBlocks(3).Has(block.ID()))

	require.NoError(t, lo.Return3(mutationFactory.Evict(3)))
}
