package messagelayer

import (
	"fmt"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"testing"
)


func MockBroadcastStatement(conflicts statement.Conflicts, timestamps statement.Timestamps) {
	fmt.Printf("Here")
}

func TestMakeStatement(t *testing.T) {
	senderSeed := walletseed.NewSeed()
	maxSize := payload.MaxSize
	maxNumOfStatements := maxSize / statement.ConflictLength

	stats := &vote.RoundStats{
		ActiveVoteContexts: map[string]*vote.Context{},
	}
	for i:=0; i<maxNumOfStatements; i++ {
		// transactionId needs to be in base58 format
		transactionID := senderSeed.Address(uint64(i)).Base58()
		// generate voting context to fill in the payload
		stats.ActiveVoteContexts[transactionID] = &vote.Context{
			ID:              "one",
			ProportionLiked: 1.,
			Rounds:          1,
			Opinions:        []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like},
		}
	}
	makeStatement(stats, MockBroadcastStatement)
}


