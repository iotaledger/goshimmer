package drng

import (
	"testing"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func TestParseHeader(t *testing.T) {
	header := NewHeader(TypeCollectiveBeacon, 0)
	bytes := header.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedHeader, err := ParseHeader(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, header, parsedHeader)
}
