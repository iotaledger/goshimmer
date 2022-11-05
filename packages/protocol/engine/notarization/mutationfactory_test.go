package notarization

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestMutationFactory_AddAcceptedBlock(t *testing.T) {
	mutationFactory := NewEpochMutations(2)

	block := models.NewBlock(
		models.WithIssuingTime(epoch.Index(3).EndTime()),
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
	)
	require.NoError(t, block.DetermineID())

	require.NoError(t, mutationFactory.AddAcceptedBlock(block))
	require.True(t, mutationFactory.acceptedBlocks(3).Has(block.ID()))

	acceptedBlocks, acceptedTransactions, activeValidators, err := mutationFactory.Commit(3)
	require.NoError(t, err)
	fmt.Println(acceptedBlocks.Root(), acceptedTransactions.Root(), activeValidators.Root())
	fmt.Println(activeValidators.Has(block.IssuerID()))
}
