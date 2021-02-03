package statement

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func dummyPayload() *Statement {
	conflicts := []Conflict{
		{transaction.RandomID(), Opinion{opinion.Like, 1}},
		{transaction.RandomID(), Opinion{opinion.Like, 2}},
	}
	timestamps := []Timestamp{
		{tangle.EmptyMessageID, Opinion{opinion.Like, 1}},
		{tangle.EmptyMessageID, Opinion{opinion.Dislike, 2}},
	}
	return New(conflicts, timestamps)
}

func emptyPayload() *Statement {
	conflicts := []Conflict{}
	timestamps := []Timestamp{}
	return New(conflicts, timestamps)
}

func TestPayloadFromMarshalUtil(t *testing.T) {
	payload := dummyPayload()
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := Parse(marshalUtil)
	require.NoError(t, err)

	require.EqualValues(t, 2, parsedPayload.ConflictsCount)
	require.EqualValues(t, 2, parsedPayload.TimestampsCount)
	require.Equal(t, payload.Conflicts, parsedPayload.Conflicts)
	require.Equal(t, payload.Timestamps, parsedPayload.Timestamps)

	fmt.Println(parsedPayload)
}

func TestEmptyPayloadFromMarshalUtil(t *testing.T) {
	payload := emptyPayload()
	bytes := payload.Bytes()

	marshalUtil := marshalutil.New(bytes)
	parsedPayload, err := Parse(marshalUtil)
	require.NoError(t, err)

	require.EqualValues(t, 0, parsedPayload.ConflictsCount)
	require.EqualValues(t, 0, parsedPayload.TimestampsCount)
	require.Equal(t, payload.Conflicts, parsedPayload.Conflicts)
	require.Equal(t, payload.Timestamps, parsedPayload.Timestamps)

	fmt.Println(parsedPayload)
}

func TestString(t *testing.T) {
	payload := dummyPayload()
	_ = payload.String()
}
