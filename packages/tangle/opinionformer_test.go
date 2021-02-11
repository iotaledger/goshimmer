package tangle

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestOpinionFormer(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()

	tangle.Setup()

	tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("Booking:", messageID)
		err := tangle.Booker.Book(messageID)
		if err != nil {
			fmt.Println(err)
		}
	}))
	var wg sync.WaitGroup

	tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("Message Booked:", messageID)
	}))

	tangle.OpinionFormer.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("MessageOpinionFormed for ", messageID)
		wg.Done()
		assert.True(t, tangle.OpinionFormer.MessageEligible(messageID))
	}))

	tangle.Events.MessageInvalid.Attach(events.NewClosure(func(messageID MessageID) {
		fmt.Println("Invalid message:", messageID)
	}))

	tangle.OpinionFormer.payloadOpinionProvider.Vote().Attach(events.NewClosure(func(transactionID string, initialOpinion opinion.Opinion) {
		fmt.Println("Voting requested")
		wg.Done()
	}))

	wg.Add(3)

	messageA := newTestDataMessage("A")

	tangle.Storage.StoreMessage(messageA)

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
	messageB := newTestParentsPayloadMessage(tx1, []MessageID{messageA.ID()}, []MessageID{})

	output2 := ledgerstate.NewSigLockedSingleOutput(10000, wallets[1].address)
	txEssence2 := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output2))
	tx2 := ledgerstate.NewTransaction(txEssence2, wallets[0].unlockBlocks(txEssence2))
	messageC := newTestParentsPayloadMessage(tx2, []MessageID{messageA.ID()}, []MessageID{})

	go tangle.Storage.StoreMessage(messageB)
	go tangle.Storage.StoreMessage(messageC)

	wg.Wait()

	fmt.Println("Waiting shhutdown..")
}
