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
	parsedpayload, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) })
	require.NoError(t, err)

	cb := parsedpayload.(*Payload)

	require.Equal(t, payload.SubType(), cb.SubType())
	require.Equal(t, payload.Instance(), cb.Instance())
	require.Equal(t, payload.Data(), cb.Data())
}
