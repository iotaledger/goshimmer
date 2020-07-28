package tangle

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	valuePayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messageparser"
	messagePayload "github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/require"
)

func TestSignatureFilter(t *testing.T) {
	// create parser
	messageParser := newSyncMessageParser(NewSignatureFilter())

	// create helper instances
	seed := newSeed()
	messageFactory := messagefactory.New(mapdb.NewMapDB(), []byte("sequenceKey"), identity.GenerateLocalIdentity(), tipselector.New())

	// 1. test value message without signatures
	{
		// create unsigned transaction
		tx := transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(0), transaction.GenesisID),
			),
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(1): {
					balance.New(balance.ColorIOTA, 1337),
				},
			}),
		)

		// parse message bytes
		msg, err := messageFactory.IssuePayload(valuePayload.New(valuePayload.GenesisID, valuePayload.GenesisID, tx))
		require.NoError(t, err)
		accepted, _, _, err := messageParser.Parse(msg.Bytes(), &peer.Peer{})

		// check results (should be rejected)
		require.Equal(t, false, accepted)
		require.NotNil(t, err)
		require.Equal(t, "invalid transaction signatures", err.Error())
	}

	// 2. test value message with signatures
	{
		// create signed transaction
		tx := transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(0), transaction.GenesisID),
			),
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(1): {
					balance.New(balance.ColorIOTA, 1337),
				},
			}),
		)
		tx.Sign(signaturescheme.ED25519(*seed.KeyPair(0)))

		// parse message bytes
		msg, err := messageFactory.IssuePayload(valuePayload.New(valuePayload.GenesisID, valuePayload.GenesisID, tx))
		require.NoError(t, err)

		accepted, _, _, err := messageParser.Parse(msg.Bytes(), &peer.Peer{})

		// check results (should be accepted)
		require.Equal(t, true, accepted)
		require.Nil(t, err)
	}

	// 3. test message with an invalid value payload
	{
		// create a data payload
		marshalUtil := marshalutil.New(messagePayload.NewData([]byte("test")).Bytes())

		// set the type to be a value payload
		marshalUtil.WriteSeek(0)
		marshalUtil.WriteUint32(valuePayload.Type)

		// parse modified bytes back into a payload object
		dataPayload, _, err := messagePayload.DataFromBytes(marshalUtil.Bytes())
		require.NoError(t, err)

		// parse message bytes
		msg, err := messageFactory.IssuePayload(dataPayload)
		require.NoError(t, err)
		accepted, _, _, err := messageParser.Parse(msg.Bytes(), &peer.Peer{})

		// check results (should be rejected)
		require.Equal(t, false, accepted)
		require.NotNil(t, err)
		require.Equal(t, "invalid value message", err.Error())
	}
}

// newSyncMessageParser creates a wrapped MessageParser that works synchronously by using a WaitGroup to wait for the
// parse result.
func newSyncMessageParser(messageFilters ...messageparser.MessageFilter) (tester *syncMessageParser) {
	// initialize MessageParser
	messageParser := messageparser.New()
	for _, messageFilter := range messageFilters {
		messageParser.AddMessageFilter(messageFilter)
	}

	// create wrapped result
	tester = &syncMessageParser{
		messageParser: messageParser,
	}

	// setup async behavior (store result + mark WaitGroup done)
	messageParser.Events.BytesRejected.Attach(events.NewClosure(func(bytes []byte, err error, peer *peer.Peer) {
		tester.result = &messageParserResult{
			accepted: false,
			message:  nil,
			peer:     peer,
			err:      err,
		}

		tester.wg.Done()
	}))
	messageParser.Events.MessageRejected.Attach(events.NewClosure(func(message *message.Message, err error, peer *peer.Peer) {
		tester.result = &messageParserResult{
			accepted: false,
			message:  message,
			peer:     peer,
			err:      err,
		}

		tester.wg.Done()
	}))
	messageParser.Events.MessageParsed.Attach(events.NewClosure(func(message *message.Message, peer *peer.Peer) {
		tester.result = &messageParserResult{
			accepted: true,
			message:  message,
			peer:     peer,
			err:      nil,
		}

		tester.wg.Done()
	}))

	return
}

// syncMessageParser is a wrapper for the MessageParser that allows to parse Messages synchronously.
type syncMessageParser struct {
	messageParser *messageparser.MessageParser
	result        *messageParserResult
	wg            sync.WaitGroup
}

// Parse parses the message bytes into a message. It either gets accepted or rejected.
func (tester *syncMessageParser) Parse(messageBytes []byte, peer *peer.Peer) (bool, *message.Message, *peer.Peer, error) {
	tester.wg.Add(1)
	tester.messageParser.Parse(messageBytes, peer)
	tester.wg.Wait()

	return tester.result.accepted, tester.result.message, tester.result.peer, tester.result.err
}

// messageParserResult is a struct that stores the results of a parsing operation, so we can return them after the
// WaitGroup is done waiting.
type messageParserResult struct {
	accepted bool
	message  *message.Message
	peer     *peer.Peer
	err      error
}
