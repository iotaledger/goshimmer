package tangle

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"golang.org/x/xerrors"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	Parser         *MessageParser
	Storage        *Storage
	Solidifier     *Solidifier
	Booker         *Booker
	TipManager     *MessageTipSelector
	Requester      *MessageRequester
	MessageFactory *MessageFactory
	LedgerState    *LedgerState
	Utils          *Utils
	Options        *Options
	Events         *Events

	setupParserOnce sync.Once
}

// New is the constructor for the Tangle.
func New(options ...Option) (tangle *Tangle) {
	tangle = emptyTangle()
	for _, option := range options {
		option(tangle.Options)
	}

	// create components
	tangle.Parser = NewMessageParser()
	tangle.Storage = NewStorage(tangle)
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Requester = NewMessageRequester(tangle)
	tangle.TipManager = NewMessageTipSelector(tangle)
	tangle.MessageFactory = NewMessageFactory(tangle.Options.Store, []byte(DBSequenceNumber), tangle.Options.Identity, tangle.TipManager)
	tangle.LedgerState = NewLedgerState(tangle)
	tangle.Utils = NewUtils(tangle)

	// setup data flow
	tangle.Storage.Setup()
	tangle.Solidifier.Setup()
	tangle.Requester.Setup()
	tangle.TipManager.Setup()

	tangle.MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		tangle.Events.Error.Trigger(xerrors.Errorf("error in MessageFactory: %w", err))
	}))

	return
}

// emptyTangle creates an un-configured Tangle without a configured data flow.
func emptyTangle() (tangle *Tangle) {
	tangle = &Tangle{
		Options: defaultOptions(),
		Events: &Events{
			MessageEligible: events.NewEvent(cachedMessageEvent),
			MessageInvalid:  events.NewEvent(messageIDEventHandler),
			Error:           events.NewEvent(events.ErrorCaller),
		},
	}

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
	fmt.Println("SHUTDOWN")
	t.MessageFactory.Shutdown()
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

	// Error is triggered when the Tangle faces an error from which it can not recover.
	Error *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Tangle to
// configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Tangle.
type Options struct {
	Store    kvstore.KVStore
	Identity *identity.LocalIdentity
}

// defaultOptions returns the default options that are used by the Tangle.
func defaultOptions() *Options {
	return &Options{
		Store:    mapdb.NewMapDB(),
		Identity: identity.GenerateLocalIdentity(),
	}
}

// Store is an Option for the Tangle that allows to specify which storage layer is supposed to be used to persist data.
func Store(store kvstore.KVStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}

// Identity is an Option for the Tangle that allows to specify the node identity which is used to issue Messages.
func Identity(identity *identity.LocalIdentity) Option {
	return func(options *Options) {
		options.Identity = identity
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
