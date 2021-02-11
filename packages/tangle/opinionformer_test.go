package tangle

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		// wg.Done()
		fmt.Println("Invalid message:", messageID)
	}))

	wg.Add(2)

	messageA := newTestDataMessage("A")

	tangle.Storage.StoreMessage(messageA)

	wallets := createWallets(1)

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

	tx := ledgerstate.NewTransaction(txEssence, wallets[0].unlockBlocks(txEssence))
	fmt.Println("ReferencedTransactionIDs", tx.ReferencedTransactionIDs())

	_, err := tangle.MessageFactory.IssuePayload(tx)
	require.NoError(t, err)

	// assert.True(t, tangle.LedgerState.TransactionValid(tx, msg.ID()))

	// fmt.Println(tangle.Booker.allTransactionsApprovedByMessage(tx.ReferencedTransactionIDs(), msg.ID()))

	wg.Wait()

}
