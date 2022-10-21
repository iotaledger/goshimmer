package chainstorage

import (
	"fmt"
	"testing"

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

	chainStorage, err := NewChainStorage(storageDirectory, 1)
	require.NoError(t, err)
	chainStorage.SetLatestAcceptedEpoch(10)
	genesisCommitment := commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	chainStorage.SetCommitment(0, genesisCommitment)
	chainStorage.SetCommitment(1, commitment.New(1, genesisCommitment.ID(), types.Identifier{}, 0))
	chainStorage.BlockStorage.Store(emptyBlock)
	fmt.Println(chainStorage.BlockStorage.Get(emptyBlock.ID()))

	chainStorage.database.Flush(0)

	chainStorage.Shutdown()

	chainStorage, err = NewChainStorage(storageDirectory, 1)
	require.NoError(t, err)
	fmt.Println(chainStorage.Commitment(0), chainStorage.Commitment(1))
	require.Equal(t, epoch.Index(10), chainStorage.LatestAcceptedEpoch())

	fmt.Println(chainStorage.BlockStorage.Get(emptyBlock.ID()))

	chainStorage.Shutdown()
}
