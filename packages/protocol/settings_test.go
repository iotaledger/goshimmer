package protocol

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettings(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "setting.bin")

	settings := NewSettings(filePath)
	require.Equal(t, [32]byte{}, settings.SnapshotChecksum())

	settings.SetSnapshotChecksum([32]byte{1})
	require.Equal(t, [32]byte{1}, settings.SnapshotChecksum())

	restoredSettings := NewSettings(filePath)
	require.Equal(t, [32]byte{1}, restoredSettings.SnapshotChecksum())
}
