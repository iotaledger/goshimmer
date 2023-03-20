package slotnotarization

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
)

func TestMutationFactory(t *testing.T) {
	tf := NewTestFramework(t, slot.NewTimeProvider(time.Now().Unix(), 10))

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

	// commit slot 1 (empty)
	tf.AssertCommit(1, nil, nil, nil, 0)

	// mutate slot 1 (errors expected)
	require.Error(t, tf.AddAcceptedBlock("1.1"))
	require.Error(t, tf.RemoveAcceptedBlock("1.1"))
	require.NoError(t, tf.UpdateTransactionInclusion("tx1.1", 3, 5))
	require.Error(t, tf.UpdateTransactionInclusion("tx3.1", 2, 1))
	tf.AssertCommit(1, nil, nil, nil, 0, true)

	// mutate slot 2
	require.NoError(t, tf.AddAcceptedBlock("2.1"))
	require.NoError(t, tf.AddAcceptedBlock("2.2"))
	require.NoError(t, tf.AddAcceptedBlock("2.3"))
	require.NoError(t, tf.AddAcceptedTransaction("tx2.1"))
	require.NoError(t, tf.AddAcceptedTransaction("tx2.2"))
	require.NoError(t, tf.RemoveAcceptedTransaction("tx2.2"))

	// mutate slot 3
	require.NoError(t, tf.AddAcceptedBlock("3.1"))
	require.NoError(t, tf.AddAcceptedBlock("3.2"))
	require.NoError(t, tf.AddAcceptedTransaction("tx3.1"))
	require.NoError(t, tf.UpdateTransactionInclusion("tx3.1", 3, 2))
	require.NoError(t, tf.RemoveAcceptedBlock("3.2"))

	// assert commitment of slot 2
	tf.AssertCommit(2, []string{"2.1", "2.2", "2.3"}, []string{"tx2.1", "tx3.1"}, []string{"Batman", "Robin", "Joker"}, 30, false)

	// assert commitment of slot 3
	// The CW is 10 because the slot mutation does not accumulate the weights, the notarization manager does.
	tf.AssertCommit(3, []string{"3.1"}, []string{}, []string{"Batman"}, 10, false)
}

func TestMutationFactory_AddAcceptedBlock(t *testing.T) {
	slotTimeProvider := slot.NewTimeProvider(time.Now().Unix(), 10)
	mutationFactory := NewSlotMutations(sybilprotection.NewWeights(mapdb.NewMapDB()), 2)

	block := models.NewBlock(
		models.WithIssuingTime(slotTimeProvider.EndTime(3)),
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
	)
	require.NoError(t, block.DetermineID(slotTimeProvider))

	require.NoError(t, mutationFactory.AddAcceptedBlock(block))
	require.True(t, mutationFactory.acceptedBlocks(3).Has(block.ID()))

	require.NoError(t, lo.Return3(mutationFactory.Evict(3)))
}
