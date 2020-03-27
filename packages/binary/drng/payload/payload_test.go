package payload

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func dummyPayload() *Payload {
	header := header.New(header.CollectiveBeaconType(), 0)
	data := []byte("test")
	return New(header, data)
}

func TestParse(t *testing.T) {
	payload := dummyPayload()
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := Parse(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, payload.SubType(), parsedPayload.SubType())
	require.Equal(t, payload.Instance(), parsedPayload.Instance())
	require.Equal(t, payload.Data(), parsedPayload.Data())
}

func TestString(t *testing.T) {
	payload := dummyPayload()
	_ = payload.String()
}
