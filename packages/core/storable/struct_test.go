package storable_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// region Tests ////////////////////////////////////////////////////////////////////////////////////////////////////////

func TestStruct(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "node.settings")

	settings := NewSettings(filePath)
	require.Equal(t, uint64(123), settings.Number)

	settings.Number = 3
	require.NoError(t, settings.ToFile())

	restoredSettings := NewSettings(filePath)
	require.Equal(t, settings.Number, restoredSettings.Number)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Settings /////////////////////////////////////////////////////////////////////////////////////////////////////

type Settings struct {
	Number uint64 `serix:"1"`

	storable.Struct[Settings, *Settings]
}

func NewSettings(filePath string) (settings *Settings) {
	return storable.InitStruct(&Settings{
		Number: 123,
	}, filePath)
}

func (t *Settings) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, t)
}

func (t *Settings) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), *t)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
