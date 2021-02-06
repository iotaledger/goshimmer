package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	Parser         *MessageParser
	Storage        *MessageStore
	Solidifier     *Solidifier
	Booker         *Booker
	Requester      *MessageRequester
	MessageFactory *MessageFactory
	MarkersManager *MarkersManager
	LedgerState    *LedgerState
	Utils          *Utils
	Events         *Events

	setupParserOnce sync.Once
}

// New is the constructor for the Tangle.
func New(store kvstore.KVStore) (tangle *Tangle) {
	tangle = &Tangle{
		Events: &Events{
			MessageEligible: events.NewEvent(cachedMessageEvent),
			MessageInvalid:  events.NewEvent(messageIDEventHandler),
		},
	}

	// create components
	tangle.Parser = NewMessageParser()
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Storage = NewMessageStore(tangle, store)
	tangle.LedgerState = NewLedgerState(tangle)
	tangle.MarkersManager = NewMarkersManager(tangle)
	tangle.Utils = NewUtils(tangle)

	// setup data flow
	tangle.Parser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		tangle.Storage.StoreMessage(msgParsedEvent.Message)
	}))
	tangle.Storage.Events.MessageStored.Attach(events.NewClosure(tangle.Solidifier.Solidify))

	return
}

// ProcessGossipMessage is used to feed new Messages from the gossip layer into the Tangle.
func (t *Tangle) ProcessGossipMessage(messageBytes []byte, peer *peer.Peer) {
	t.setupParserOnce.Do(t.Parser.Setup)

	t.Parser.Parse(messageBytes, peer)
}

// Prune resets the database and deletes all stored objects (good for testing or "node resets").
func (t *Tangle) Prune() (err error) {
	return t.Storage.Prune()
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.Storage.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Tangle.
type Events struct {
	// MessageInvalid is triggered when a Message is detected to be objectively invalid.
	MessageInvalid *events.Event

	// Fired when a message has been eligible.
	MessageEligible *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

type Option func(*Tangle)

type Options struct {
}

func PoW(interval time.Duration) MessageRequesterOption {
	return func(args *MessageRequesterOptions) {
		args.retryInterval = interval
	}
}

func PoWDifficulty(difficulty int) Option {
	return func(tangle *Tangle) {

	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
