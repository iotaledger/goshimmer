package commitment

import (
	"testing"

	"github.com/iotaledger/hive.go/ds/types"
	"github.com/stretchr/testify/require"
)

func TestCommitment_RootsSerialization(t *testing.T) {
	var (
		r             *Roots
		rDeserialized Roots
		rootsID       types.Identifier
	)

	err := rootsID.FromRandomness()
	require.NoError(t, err)

	r = NewRoots(rootsID, rootsID, rootsID, rootsID, rootsID)
	require.Equal(t, rootsID, r.ActivityRoot())
	require.Equal(t, rootsID, r.TangleRoot())
	require.Equal(t, rootsID, r.ManaRoot())
	require.Equal(t, rootsID, r.StateMutationRoot())
	require.Equal(t, rootsID, r.StateRoot())

	bytes, err := r.Bytes()
	require.NoError(t, err)

	_, err = rDeserialized.FromBytes(bytes)
	require.NoError(t, err)
	require.EqualValues(t, *r, rDeserialized)
	require.Equal(t, r.ID(), rDeserialized.ID())
}
