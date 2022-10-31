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

	chainStorage, err := New(storageDirectory, 1)
	require.NoError(t, err)
	chainStorage.Settings.SetLatestStateMutationEpoch(10)
	genesisCommitment := commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	chainStorage.Commitments.Store(0, genesisCommitment)
	chainStorage.Commitments.Store(1, commitment.New(1, genesisCommitment.ID(), types.Identifier{}, 0))
	chainStorage.Blocks.Store(emptyBlock)
	fmt.Println(chainStorage.Blocks.Load(emptyBlock.ID()))

	chainStorage.database.Flush(0)

	chainStorage.Shutdown()

	chainStorage, err = New(storageDirectory, 1)
	require.NoError(t, err)
	fmt.Println(lo.PanicOnErr(chainStorage.Commitments.Load(0)), lo.PanicOnErr(chainStorage.Commitments.Load(1)))
	require.Equal(t, epoch.Index(10), chainStorage.Settings.LatestStateMutationEpoch())

	fmt.Println(chainStorage.Blocks.Load(emptyBlock.ID()))

	chainStorage.Shutdown()
}
