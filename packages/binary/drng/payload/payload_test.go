package payload

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	header := header.New(header.CollectiveBeaconType(), 0)
	data := []byte("test")
	payload := New(header, data)
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := Parse(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, payload.SubType(), parsedPayload.SubType())
	require.Equal(t, payload.Instance(), parsedPayload.Instance())
	require.Equal(t, payload.Data(), parsedPayload.Data())
}
