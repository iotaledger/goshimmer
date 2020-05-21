package ledgerstate

import (
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/consensus"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/testutil"
)

func TestTangle_ValueTransfer(t *testing.T) {
	// initialize tangle + ledgerstate
	valueTangle := tangle.New(testutil.DB(t))
	consensus.NewFCOB(valueTangle, 0)
	if err := valueTangle.Prune(); err != nil {
		t.Error(err)

		return
	}
	ledgerState := tangle.NewLedgerState(valueTangle)

	// generate addresses and key
	addressKeyPair1 := ed25519.GenerateKeyPair()
	addressKeyPair2 := ed25519.GenerateKeyPair()
	address1 := address.FromED25519PubKey(addressKeyPair1.PublicKey)
	address2 := address.FromED25519PubKey(addressKeyPair2.PublicKey)

	// check if ledger empty first
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address1))
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address2))

	// load snapshot
	valueTangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			address1: []*balance.Balance{
				balance.New(balance.ColorIOTA, 337),
			},

			address2: []*balance.Balance{
				balance.New(balance.ColorIOTA, 1000),
			},
		},
	})

	// check if balance exists after loading snapshot
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 337}, ledgerState.Balances(address1))
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 1000}, ledgerState.Balances(address2))

	// introduce logic to record liked payloads
	recordedLikedPayloads := make(map[payload.ID]types.Empty)
	valueTangle.Events.PayloadLiked.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *tangle.CachedPayloadMetadata) {
		defer cachedPayloadMetadata.Release()

		cachedPayload.Consume(func(payload *payload.Payload) {
			recordedLikedPayloads[payload.ID()] = types.Void
		})
	}))
	resetRecordedLikedPayloads := func() {
		recordedLikedPayloads = make(map[payload.ID]types.Empty)
	}

	// attach first spend
	outputAddress1 := address.Random()
	attachedPayload1 := payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(address1, transaction.GenesisID),
			transaction.NewOutputID(address2, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress1: {
				balance.New(balance.ColorIOTA, 1337),
			},
		}),
	))
	valueTangle.AttachPayloadSync(attachedPayload1)

	// check if old addresses are empty and new addresses are filled
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address1))
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address2))
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 1337}, ledgerState.Balances(outputAddress1))
	assert.Equal(t, 1, len(recordedLikedPayloads))
	_, payloadLiked := recordedLikedPayloads[attachedPayload1.ID()]
	assert.True(t, payloadLiked)

	resetRecordedLikedPayloads()

	// attach double spend
	outputAddress2 := address.Random()
	valueTangle.AttachPayloadSync(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(address1, transaction.GenesisID),
			transaction.NewOutputID(address2, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress2: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))

	// shutdown tangle
	valueTangle.Shutdown()
}
