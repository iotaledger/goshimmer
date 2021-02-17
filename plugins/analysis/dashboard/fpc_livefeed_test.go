package dashboard

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	analysisserver "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/stretchr/testify/require"
)

// TestCreateFPCUpdate checks that given a FPC heartbeat, the returned FPCUpdate is ok.
func TestCreateFPCUpdate(t *testing.T) {
	ownID := sha256.Sum256([]byte{'A'})
	base58OwnID := analysisserver.ShortNodeIDString(ownID[:])

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
					Opinions: []opinion.Opinion{opinion.Dislike, opinion.Like, opinion.Dislike},
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
	for k, v := range createFPCUpdate(hbTest).Conflicts {
		require.Equal(t, want.Conflicts[k].NodesView, v.NodesView)
	}

}
