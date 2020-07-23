package test

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/types"

	"github.com/stretchr/testify/assert"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/consensus"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

func TestTangle_ValueTransfer(t *testing.T) {
	// initialize tangle
	valueTangle := tangle.New(mapdb.NewMapDB())
	defer valueTangle.Shutdown()

	// initialize ledger state
	ledgerState := tangle.NewLedgerState(valueTangle)

	// initialize seed
	seed := walletseed.NewSeed()

	// setup consensus rules
	consensus.NewFCOB(valueTangle, 0)

	// check if ledger empty first
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(seed.Address(0).Address))
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(seed.Address(1).Address))

	// load snapshot
	valueTangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			seed.Address(0).Address: []*balance.Balance{
				balance.New(balance.ColorIOTA, 337),
			},

			seed.Address(1).Address: []*balance.Balance{
				balance.New(balance.ColorIOTA, 1000),
			},
		},
	})

	// check if balance exists after loading snapshot
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 337}, ledgerState.Balances(seed.Address(0).Address))
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 1000}, ledgerState.Balances(seed.Address(1).Address))

	// introduce logic to record liked payloads
	recordedLikedPayloads, resetRecordedLikedPayloads := recordLikedPayloads(valueTangle)

	// attach first spend
	outputAddress1 := address.Random()
	attachedPayload1 := payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(seed.Address(0).Address, transaction.GenesisID),
			transaction.NewOutputID(seed.Address(1).Address, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress1: {
				balance.New(balance.ColorIOTA, 1337),
			},
		}),
	))
	valueTangle.AttachPayloadSync(attachedPayload1)

	valueTangle.PayloadMetadata(attachedPayload1.ID()).Consume(func(payload *tangle.PayloadMetadata) {
		fmt.Println(payload.Confirmed())
	})

	// check if old addresses are empty and new addresses are filled
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(seed.Address(0).Address))
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(seed.Address(1).Address))
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 1337}, ledgerState.Balances(outputAddress1))
	assert.Equal(t, 1, len(recordedLikedPayloads))
	assert.Contains(t, recordedLikedPayloads, attachedPayload1.ID())

	resetRecordedLikedPayloads()

	// attach double spend
	outputAddress2 := address.Random()
	valueTangle.AttachPayloadSync(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(seed.Address(0).Address, transaction.GenesisID),
			transaction.NewOutputID(seed.Address(1).Address, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress2: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))
}

func recordLikedPayloads(valueTangle *tangle.Tangle) (recordedLikedPayloads map[payload.ID]types.Empty, resetFunc func()) {
	recordedLikedPayloads = make(map[payload.ID]types.Empty)

	valueTangle.Events.PayloadLiked.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *tangle.CachedPayloadMetadata) {
		defer cachedPayloadMetadata.Release()

		cachedPayload.Consume(func(payload *payload.Payload) {
			recordedLikedPayloads[payload.ID()] = types.Void
		})
	}))

	resetFunc = func() {
		recordedLikedPayloads = make(map[payload.ID]types.Empty)
	}

	return
}
