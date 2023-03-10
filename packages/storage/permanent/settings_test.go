package permanent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/ds/types"
)

func TestSettings_Serialization(t *testing.T) {
	tempDir := utils.NewDirectory(t.TempDir())
	path := tempDir.Path("settings.bin")

	settings := NewSettings(path)
	require.NoError(t, settings.SetSnapshotImported(true))
	require.NoError(t, settings.SetGenesisUnixTime(12345678))
	require.NoError(t, settings.SetSlotDuration(99))
	require.NoError(t, settings.SetLatestCommitment(commitment.New(7, commitment.NewID(6, []byte("test")), types.NewIdentifier([]byte("foo")), 666)))
	require.NoError(t, settings.SetLatestStateMutationSlot(23))
	require.NoError(t, settings.SetLatestConfirmedSlot(15))
	require.NoError(t, settings.SetChainID(commitment.NewEmptyCommitment().ID()))

	require.NoError(t, settings.ToFile())

	settings2 := NewSettings(path)
	require.NoError(t, settings2.FromFile(path))

	require.Equal(t, settings.settingsModel, settings2.settingsModel)
	require.Equal(t, settings.slotTimeProvider, settings2.slotTimeProvider)

	filePath := tempDir.Path("exported.bin")
	fileHandle, err := os.Create(filePath)
	require.NoError(t, err)
	require.NoError(t, settings.Export(fileHandle))
	require.NoError(t, fileHandle.Close())

	fileHandle2, err := os.Open(filePath)
	require.NoError(t, err)
	imported := NewSettings(tempDir.Path("otherfile.bin"))
	require.NoError(t, imported.Import(fileHandle2))
	require.NoError(t, fileHandle2.Close())

	// We cannot check with equality of the whole settings because we have a different filePath here
	require.Equal(t, settings.SnapshotImported(), imported.SnapshotImported())
	require.Equal(t, settings.GenesisUnixTime(), imported.GenesisUnixTime())
	require.Equal(t, settings.SlotDuration(), imported.SlotDuration())
	require.Equal(t, settings.LatestCommitment(), imported.LatestCommitment())
	require.Equal(t, settings.LatestStateMutationSlot(), imported.LatestStateMutationSlot())
	require.Equal(t, settings.LatestConfirmedSlot(), imported.LatestConfirmedSlot())
	require.Equal(t, settings.ChainID(), imported.ChainID())
}
