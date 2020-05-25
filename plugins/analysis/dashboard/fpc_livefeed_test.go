package dashboard

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/stretchr/testify/require"
)

// TestCreateFPCUpdate checks that given a FPC heartbeat, the returned FPCUpdate is ok.
func TestCreateFPCUpdate(t *testing.T) {
	ownID := sha256.Sum256([]byte{'A'})
	shortOwnID := fmt.Sprintf("%x", ownID[:8])

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
		Finalized: map[string]vote.Opinion{"one": vote.Like},
	}

	// create a matching FPCUpdate
	want := &FPCUpdate{
		Conflicts: map[string]Conflict{
			"one": {
				NodesView: map[string]voteContext{
					shortOwnID: {
						NodeID:   shortOwnID,
						Rounds:   3,
						Opinions: []int32{disliked, liked, disliked},
						Status:   liked,
					},
				},
			},
		},
	}

	// check that createFPCUpdate returns a matching FPCMsg
	require.Equal(t, want, createFPCUpdate(hbTest))

}
