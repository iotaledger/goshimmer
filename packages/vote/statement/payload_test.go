package statement

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

func dummyPayload(t *testing.T) *Statement {
	txA, err := ledgerstate.TransactionIDFromRandomness()
	require.NoError(t, err)
	txB, err := ledgerstate.TransactionIDFromRandomness()
	require.NoError(t, err)
	conflicts := []Conflict{
		{txA, Opinion{opinion.Like, 1}},
		{txB, Opinion{opinion.Like, 2}},
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
	payload := dummyPayload(t)
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
	payload := dummyPayload(t)
	_ = payload.String()
}
