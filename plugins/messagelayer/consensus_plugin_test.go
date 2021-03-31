package messagelayer

import (
	"fmt"
	"testing"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
)

func MockBroadcastStatement(conflicts statement.Conflicts, timestamps statement.Timestamps) {
	statementPayload := statement.New(conflicts, timestamps)
	payloadLen := len(statementPayload.Bytes())
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		panic(err)
	}
}

func TestMakeStatement(t *testing.T) {
	senderSeed := walletseed.NewSeed()
	maxSize := payload.MaxSize
	maxNumOfStatements := maxSize * 4 / statement.ConflictLength

	stats := &vote.RoundStats{
		ActiveVoteContexts: map[string]*vote.Context{},
	}
	for i := 0; i < maxNumOfStatements; i++ {
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
	// if max payload size exceeded MockBroadcastStatement will panic
	makeStatement(stats, MockBroadcastStatement)
}
