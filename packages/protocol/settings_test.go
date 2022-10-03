package protocol

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

func TestSettings(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "setting.bin")

	settings := NewSettings(filePath)
	require.Equal(t, commitment.ID{}, settings.MainChainID())

	mainChainID := commitment.NewID(1, []byte{1, 2, 3})
	settings.SetMainChainID(mainChainID)
	require.Equal(t, mainChainID, settings.MainChainID())
	settings.Persist()

	restoredSettings := NewSettings(filePath)
	require.Equal(t, mainChainID, restoredSettings.MainChainID())
}
