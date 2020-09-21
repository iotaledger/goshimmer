package drng

import (
	"testing"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func dummyPayload() *Payload {
	header := NewHeader(TypeCollectiveBeacon, 0)
	data := []byte("test")
	return NewPayload(header, data)
}

func TestPayloadFromMarshalUtil(t *testing.T) {
	payload := dummyPayload()
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := PayloadFromMarshalUtil(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, payload.Header.PayloadType, parsedPayload.Header.PayloadType)
	require.Equal(t, payload.Header.InstanceID, parsedPayload.Header.InstanceID)
	require.Equal(t, payload.Data, parsedPayload.Data)
}

func TestString(t *testing.T) {
	payload := dummyPayload()
	_ = payload.String()
}
