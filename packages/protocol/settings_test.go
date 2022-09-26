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
	require.Equal(t, commitment.EmptyMerkleRoot, settings.MainChainID())

	settings.SetMainChainID([32]byte{1})
	require.Equal(t, commitment.MerkleRoot{1}, settings.MainChainID())
	settings.Persist()

	restoredSettings := NewSettings(filePath)
	require.Equal(t, commitment.MerkleRoot{1}, restoredSettings.MainChainID())
}
