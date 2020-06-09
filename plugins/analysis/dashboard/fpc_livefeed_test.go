package dashboard

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/require"
)

// TestCreateFPCUpdate checks that given a FPC heartbeat, the returned FPCUpdate is ok.
func TestCreateFPCUpdate(t *testing.T) {
	ownID := sha256.Sum256([]byte{'A'})
	base58OwnID := base58.Encode(ownID[:])

	// create a FPCHeartbeat
	hbTest := &packet.FPCHeartbeat{
		OwnID: ownID[:],
		RoundStats: vote.RoundStats{
			Duration: time.Second,
			RandUsed: 0.5,
			ActiveVoteContexts: map[string]*vote.Context{
				"one": {
					ID:       "one",
					Liked:    1.,
					Rounds:   3,
					Opinions: []vote.Opinion{vote.Dislike, vote.Like, vote.Dislike},
				}},
		},
	}

	// create a matching FPCUpdate
	want := &FPCUpdate{
		Conflicts: conflictSet{
			"one": {
				NodesView: map[string]voteContext{
					base58OwnID: {
						NodeID:   base58OwnID,
						Rounds:   3,
						Opinions: []int32{disliked, liked, disliked},
					},
				},
			},
		},
	}

	// check that createFPCUpdate returns a matching FPCMsg
	require.Equal(t, want, createFPCUpdate(hbTest, false))

}
