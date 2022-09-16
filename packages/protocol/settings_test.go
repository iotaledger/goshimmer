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
	require.Equal(t, commitment.ID{0}, settings.ChainID())

	settings.SetChainID(commitment.ID{7})
	require.Equal(t, commitment.ID{7}, settings.ChainID())

	restoredSettings := NewSettings(filePath)
	require.Equal(t, commitment.ID{7}, restoredSettings.ChainID())
}
