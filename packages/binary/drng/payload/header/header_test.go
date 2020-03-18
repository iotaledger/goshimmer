package header

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	header := New(CollectiveBeaconType(), 0)
	bytes := header.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedHeader, err := Parse(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, header, parsedHeader)
}
