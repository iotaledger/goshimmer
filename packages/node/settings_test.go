package node

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettings(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "setting.bin")

	settings := NewSettings(filePath)
	require.Equal(t, uint64(0), settings.LatestCleanEpoch())
}
