package statement

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func dummyPayload() *Payload {
	conflicts := []Conflict{
		{transaction.RandomID(), true},
		{transaction.RandomID(), false},
	}
	timestamps := []Timestamp{
		{tangle.EmptyMessageID, true},
		{tangle.EmptyMessageID, false},
	}
	return NewPayload(conflicts, timestamps)
}

func TestPayloadFromMarshalUtil(t *testing.T) {
	payload := dummyPayload()
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := PayloadFromMarshalUtil(marshalUtil)
	require.NoError(t, err)

	require.Equal(t, uint32(2), parsedPayload.ConflictsLen)
	require.Equal(t, uint32(2), parsedPayload.TimestampsLen)
	require.Equal(t, payload.Conflicts, parsedPayload.Conflicts)
	require.Equal(t, payload.Timestamps, parsedPayload.Timestamps)

	fmt.Println(parsedPayload)
}

func TestString(t *testing.T) {
	payload := dummyPayload()
	_ = payload.String()
}
