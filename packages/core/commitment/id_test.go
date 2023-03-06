package commitment

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/stretchr/testify/require"
)

func TestCommitment_ID_Serialization(t *testing.T) {
	var (
		id             ID
		idDeserialized ID
		idFromBase58   ID
	)

	err := id.FromRandomness(2)
	require.NoError(t, err)

	require.Equal(t, slot.Index(2), id.Index())

	serializedBytes, err := id.Bytes()
	require.NoError(t, err)

	decodedBytes, err := idDeserialized.FromBytes(serializedBytes)
	require.NoError(t, err)
	require.Equal(t, len(serializedBytes), decodedBytes)
	require.EqualValues(t, id, idDeserialized)

	err = idFromBase58.FromBase58(id.Base58())
	require.NoError(t, err)
	require.EqualValues(t, id, idFromBase58)
}

func TestCommitment_ID_JSON(t *testing.T) {
	var (
		id             ID
		idDeserialized ID
	)

	err := id.FromRandomness(2)
	require.NoError(t, err)

	out, err := id.EncodeJSON()
	require.NoError(t, err)

	err = idDeserialized.DecodeJSON(out)
	require.NoError(t, err)
	require.EqualValues(t, id, idDeserialized)
}

func TestCommitment_ID_Alias(t *testing.T) {
	var (
		id         ID
		emptyAlias string
	)

	err := id.FromRandomness(2)
	require.NoError(t, err)
	emptyAlias = fmt.Sprintf("%d::%s", int(id.SlotIndex), id.Identifier.Base58())

	require.Equal(t, emptyAlias, id.Alias())

	id.RegisterAlias("A-1")
	require.Equal(t, "A-1", id.Alias())

	id.UnregisterAlias()
	require.Equal(t, emptyAlias, id.Alias())
}
