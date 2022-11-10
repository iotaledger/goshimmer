package notarization

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/identity"
)

func TestMutationFactory_AddAcceptedBlock(t *testing.T) {
	mutationFactory := NewEpochMutations(func(id identity.ID) (int64, bool) { return 1, true }, 2)

	block := models.NewBlock(
		models.WithIssuingTime(epoch.Index(3).EndTime()),
		models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)),
	)
	require.NoError(t, block.DetermineID())

	require.NoError(t, mutationFactory.AddAcceptedBlock(block))
	require.True(t, mutationFactory.acceptedBlocks(3).Has(block.ID()))

	acceptedBlocks, acceptedTransactions, activeValidators, _, err := mutationFactory.Commit(3)
	require.NoError(t, err)
	fmt.Println(acceptedBlocks.Root(), acceptedTransactions.Root(), activeValidators.Root())
	fmt.Println(activeValidators.Has(block.IssuerID()))
}
