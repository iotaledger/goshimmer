package statement

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	v := r.NodeView(identity.GenerateIdentity().ID())

	txA := transaction.RandomID()
	txB := transaction.RandomID()

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
