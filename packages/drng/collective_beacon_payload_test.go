package drng

import (
	"testing"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func dummyCollectiveBeaconPayload() *CollectiveBeaconPayload {
	header := NewHeader(TypeCollectiveBeacon, 0)
	return NewCollectiveBeaconPayload(header.InstanceID,
		0,
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), // prevSignature
		[]byte("BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"), // signature
		[]byte("CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"))                                                 // distributed PK
}

func TestParse(t *testing.T) {
	payload := dummyCollectiveBeaconPayload()
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := CollectiveBeaconPayloadFromMarshalUtil(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, payload.Header.PayloadType, parsedPayload.Header.PayloadType)
	require.Equal(t, payload.Header.InstanceID, parsedPayload.Header.InstanceID)
	require.Equal(t, payload.Round, parsedPayload.Round)
	require.Equal(t, payload.PrevSignature, parsedPayload.PrevSignature)
	require.Equal(t, payload.Signature, parsedPayload.Signature)
	require.Equal(t, payload.Dpk, parsedPayload.Dpk)
}

func TestCollectiveBeaconPayloadString(t *testing.T) {
	payload := dummyCollectiveBeaconPayload()
	_ = payload.String()
}
