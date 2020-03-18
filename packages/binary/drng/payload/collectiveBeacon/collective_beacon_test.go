package collectiveBeacon

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	header := header.New(header.CollectiveBeaconType(), 0)
	payload := New(header.Instance(),
		0,
		[]byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"), // prevSignature
		[]byte("BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"), // signature
		[]byte("CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC")) // distributed PK
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedpayload, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) })
	require.NoError(t, err)

	cb := parsedpayload.(*Payload)

	require.Equal(t, payload.SubType(), cb.SubType())
	require.Equal(t, payload.Instance(), cb.Instance())
	require.Equal(t, payload.Round(), cb.Round())
	require.Equal(t, payload.PrevSignature(), cb.PrevSignature())
	require.Equal(t, payload.Signature(), cb.Signature())
	require.Equal(t, payload.DistributedPK(), cb.DistributedPK())
}
