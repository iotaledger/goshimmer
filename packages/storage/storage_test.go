package storage

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func Test(t *testing.T) {
	storageDirectory := t.TempDir()

	emptyBlock := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)))
	require.NoError(t, emptyBlock.DetermineID())

	storage := New(storageDirectory, 1)
	storage.Settings.SetLatestStateMutationEpoch(10)
	genesisCommitment := commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	storage.Commitments.Store(0, genesisCommitment)
	storage.Commitments.Store(1, commitment.New(1, genesisCommitment.ID(), types.Identifier{}, 0))
	storage.Blocks.Store(emptyBlock)
	fmt.Println(storage.Blocks.Load(emptyBlock.ID()))

	storage.database.Flush(0)

	storage.Shutdown()

	storage = New(storageDirectory, 1)
	fmt.Println(lo.PanicOnErr(storage.Commitments.Load(0)), lo.PanicOnErr(storage.Commitments.Load(1)))
	require.Equal(t, epoch.Index(10), storage.Settings.LatestStateMutationEpoch())

	fmt.Println(storage.Blocks.Load(emptyBlock.ID()))

	storage.Shutdown()
}
