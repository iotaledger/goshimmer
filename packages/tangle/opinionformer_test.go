package tangle

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestOpinionFormer(t *testing.T) {
	LikedThreshold = 1 * time.Second
	LocallyFinalizedThreshold = 2 * time.Second

	tangle := New()
	defer tangle.Shutdown()

	messageA := newTestDataMessage("A")

	wallets := createWallets(2)

	snapshot := map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances{
		ledgerstate.GenesisTransactionID: {
			wallets[0].address: ledgerstate.NewColoredBalances(
				map[ledgerstate.Color]uint64{
					ledgerstate.ColorIOTA: 10000,
				})},
	}

	tangle.LedgerState.LoadSnapshot(snapshot)

	input := ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	output := ledgerstate.NewSigLockedSingleOutput(10000, wallets[0].address)
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output))
	tx1 := ledgerstate.NewTransaction(txEssence, wallets[0].unlockBlocks(txEssence))
	t.Log("Transacion1: ", tx1.ID())
	messageB := newTestParentsPayloadMessage(tx1, []MessageID{messageA.ID()}, []MessageID{})

	output2 := ledgerstate.NewSigLockedSingleOutput(10000, wallets[1].address)
	txEssence2 := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output2))
	tx2 := ledgerstate.NewTransaction(txEssence2, wallets[0].unlockBlocks(txEssence2))
	t.Log("Transacion2: ", tx2.ID())
	messageC := newTestParentsPayloadMessage(tx2, []MessageID{messageA.ID()}, []MessageID{})

	payloadLiked := make(map[MessageID]bool)
	payloadLiked[messageA.ID()] = true
	payloadLiked[messageB.ID()] = false
	payloadLiked[messageC.ID()] = false

	tangle.Setup()

	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Booking:", messageID)
		err := tangle.Booker.Book(messageID)
		if err != nil {
			t.Log(err)
		}
	}))
	var wg sync.WaitGroup

	tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Message Booked:", messageID)
	}))

	tangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("Invalid message:", messageID)
	}))

	tangle.OpinionFormer.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
		t.Log("MessageOpinionFormed for ", messageID)
		wg.Done()
		assert.True(t, tangle.OpinionFormer.MessageEligible(messageID))
		assert.Equal(t, payloadLiked[messageID], tangle.OpinionFormer.PayloadLiked(messageID))
		t.Log("Payload Liked:", tangle.OpinionFormer.PayloadLiked(messageID))
	}))

	tangle.OpinionFormer.payloadOpinionProvider.Vote().Attach(events.NewClosure(func(transactionID string, initialOpinion opinion.Opinion) {
		t.Log("Voting requested for:", transactionID)
		tangle.OpinionFormer.payloadOpinionProvider.ProcessVote(&vote.OpinionEvent{
			ID:      transactionID,
			Opinion: opinion.Dislike,
			Ctx:     vote.Context{Type: vote.ConflictType}})
	}))

	tangle.OpinionFormer.payloadOpinionProvider.VoteError().Attach(events.NewClosure(func(err error) {
		t.Log("VoteError", err)
	}))

	wg.Add(3)

	tangle.Storage.StoreMessage(messageA)
	tangle.Storage.StoreMessage(messageB)
	tangle.Storage.StoreMessage(messageC)

	wg.Wait()

	t.Log("Waiting shutdown..")
}
