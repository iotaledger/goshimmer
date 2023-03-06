package commitment

import (
	"testing"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/stretchr/testify/require"
)

func TestCommitment_EmptyCommitment(t *testing.T) {
	var (
		c             *Commitment
		cDeserialized Commitment
	)

	c = NewEmptyCommitment()
	bytes, err := c.Bytes()
	require.NoError(t, err)

	_, err = cDeserialized.FromBytes(bytes)
	require.NoError(t, err)
	require.EqualValues(t, *c, cDeserialized)
	require.Equal(t, c.ID(), cDeserialized.ID())
}

func TestCommitment_NewCommitment(t *testing.T) {
	var (
		index         = slot.Index(2)
		weight        = int64(10000000000)
		prevID        ID
		rootsID       types.Identifier
		c             *Commitment
		cDeserialized Commitment
	)

	err := prevID.FromRandomness(index)
	require.NoError(t, err)
	err = rootsID.FromRandomness()
	require.NoError(t, err)

	c = New(index, prevID, rootsID, weight)
	require.Equal(t, index, c.Index())
	require.Equal(t, prevID, c.PrevID())
	require.Equal(t, rootsID, c.RootsID())
	require.Equal(t, weight, c.CumulativeWeight())

	bytes, err := c.Bytes()
	require.NoError(t, err)

	_, err = cDeserialized.FromBytes(bytes)
	require.NoError(t, err)
	require.EqualValues(t, *c, cDeserialized)
	require.Equal(t, c.ID(), cDeserialized.ID())
}
