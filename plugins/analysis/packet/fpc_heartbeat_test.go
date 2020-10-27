package packet

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
	"github.com/stretchr/testify/require"
)

var ownID = sha256.Sum256([]byte{'A'})

func dummyFPCHeartbeat() *FPCHeartbeat {
	return &FPCHeartbeat{
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
			QueriedOpinions: []vote.QueriedOpinions{{
				OpinionGiverID: "nodeA",
				Opinions:       map[string]vote.Opinion{"one": vote.Like, "two": vote.Dislike},
				TimesCounted:   2,
			}},
		},
		Finalized: map[string]vote.Opinion{"one": vote.Like, "two": vote.Dislike},
	}
}

func TestFPCHeartbeat(t *testing.T) {
	hb := dummyFPCHeartbeat()

	packet, err := hb.Bytes()
	require.NoError(t, err)

	_, err = ParseFPCHeartbeat(packet)
	require.Error(t, err)

	hb.Version = banner.SimplifiedAppVersion
	packet, err = hb.Bytes()
	require.NoError(t, err)

	hbParsed, err := ParseFPCHeartbeat(packet)
	require.NoError(t, err)

	require.Equal(t, hb, hbParsed)

	tlvHeaderLength := int(tlv.HeaderMessageDefinition.MaxBytesLength)
	msg, err := NewFPCHeartbeatMessage(hb)
	require.NoError(t, err)

	require.Equal(t, MessageTypeFPCHeartbeat, message.Type(msg[0]))

	hbParsed, err = ParseFPCHeartbeat(msg[tlvHeaderLength:])
	require.NoError(t, err)
	require.Equal(t, hb, hbParsed)
}
