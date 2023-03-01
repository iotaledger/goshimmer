package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
)

func Test(t *testing.T) {
	storageDirectory := t.TempDir()

	slotTimeProvider := slot.NewTimeProvider(time.Now().Unix(), 10)
	emptyBlock := models.NewBlock(models.WithStrongParents(models.NewBlockIDs(models.EmptyBlockID)))
	require.NoError(t, emptyBlock.DetermineID(slotTimeProvider))

	storage := New(storageDirectory, 1)
	storage.Settings.SetLatestStateMutationSlot(10)
	genesisCommitment := commitment.New(0, commitment.ID{}, types.Identifier{}, 0)
	storage.Commitments.Store(genesisCommitment)
	storage.Commitments.Store(commitment.New(1, genesisCommitment.ID(), types.Identifier{}, 0))
	storage.Blocks.Store(emptyBlock)
	fmt.Println(storage.Blocks.Load(emptyBlock.ID()))

	storage.databaseManager.Flush(0)

	storage.Shutdown()

	storage = New(storageDirectory, 1)
	fmt.Println(lo.PanicOnErr(storage.Commitments.Load(0)), lo.PanicOnErr(storage.Commitments.Load(1)))
	require.Equal(t, slot.Index(10), storage.Settings.LatestStateMutationSlot())

	fmt.Println(storage.Blocks.Load(emptyBlock.ID()))

	storage.Shutdown()
}
