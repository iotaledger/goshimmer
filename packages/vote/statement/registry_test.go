package statement

import (
	"testing"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	v := r.NodeView(identity.GenerateIdentity().ID())

	txA, err := ledgerstate.TransactionIDFromRandomness()
	require.NoError(t, err)
	txB, err := ledgerstate.TransactionIDFromRandomness()
	require.NoError(t, err)

	tA := tangle.EmptyMessageID

	v.AddConflict(Conflict{txA, Opinion{opinion.Like, 1}})
	v.AddConflict(Conflict{txA, Opinion{opinion.Like, 2}})
	v.AddConflict(Conflict{txB, Opinion{opinion.Like, 1}})

	opinions := v.ConflictOpinion(txA)
	assert.Equal(t, 2, len(opinions))
	assert.Equal(t, Opinion{opinion.Like, 2}, opinions.Last())
	assert.Equal(t, true, opinions.Finalized(2))

	v.AddTimestamp(Timestamp{tA, Opinion{opinion.Like, 1}})
	o := v.TimestampOpinion(tA)
	assert.Equal(t, 1, len(o))
	assert.Equal(t, false, o.Finalized(2))
}
