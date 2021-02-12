package tangle

import (
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
	Parser         *Parser
	Storage        *Storage
	Solidifier     *Solidifier
	Scheduler      *Scheduler
	Booker         *Booker
	TipManager     *TipManager
	Requester      *Requester
	MessageFactory *MessageFactory
	LedgerState    *LedgerState
	Utils          *Utils
	Options        *Options
	Events         *Events

	OpinionFormer            *OpinionFormer
	PayloadOpinionProvider   OpinionVoterProvider
	TimestampOpinionProvider OpinionProvider

	setupParserOnce sync.Once
}

// New is the constructor for the Tangle.
func New(options ...Option) (tangle *Tangle) {
	tangle = &Tangle{
		Options: buildOptions(options...),
		Events: &Events{
			MessageEligible: events.NewEvent(messageIDEventHandler),
			MessageInvalid:  events.NewEvent(messageIDEventHandler),
			Error:           events.NewEvent(events.ErrorCaller),
		},
	}

	tangle.Parser = NewParser()
	tangle.Storage = NewStorage(tangle)
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Scheduler = NewScheduler(tangle)
	tangle.LedgerState = NewLedgerState(tangle)
	tangle.Booker = NewBooker(tangle)
	tangle.Requester = NewRequester(tangle)
	tangle.TipManager = NewTipManager(tangle)
	tangle.MessageFactory = NewMessageFactory(tangle, tangle.TipManager)
	tangle.Utils = NewUtils(tangle)

	if !tangle.Options.WithoutOpinionFormer {
		tangle.PayloadOpinionProvider = NewFCoB(tangle.Options.Store, tangle)
		tangle.TimestampOpinionProvider = NewTimestampLikedByDefault(tangle)
		tangle.OpinionFormer = NewOpinionFormer(tangle, tangle.PayloadOpinionProvider, tangle.TimestampOpinionProvider)
	}
	return
}

// Setup sets up the data flow by connecting the different components (by calling their corresponding Setup method).
func (t *Tangle) Setup() {
	t.Storage.Setup()
	t.Solidifier.Setup()
	t.Requester.Setup()
	t.TipManager.Setup()
	t.Scheduler.Setup()

	// Booker and LedgerState setup is left out until the old value tangle is in use.
	if !t.Options.WithoutOpinionFormer {
		t.OpinionFormer.Setup()
		return
	}
	t.MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(xerrors.Errorf("error in MessageFactory: %w", err))
	}))
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
	t.MessageFactory.Shutdown()
	if !t.Options.WithoutOpinionFormer {
		t.OpinionFormer.Shutdown()
	}
	t.Booker.Shutdown()
	t.LedgerState.Shutdown()
	t.Scheduler.Shutdown()
	t.Storage.Shutdown()
	t.Options.Store.Shutdown()
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

func messageIDEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(MessageID))(params[0].(MessageID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Tangle to
// configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Tangle.
type Options struct {
	Store                kvstore.KVStore
	Identity             *identity.LocalIdentity
	WithoutOpinionFormer bool
}

// buildOptions generates the Options object use by the Tangle.
func buildOptions(options ...Option) (builtOptions *Options) {
	builtOptions = &Options{
		Store:    mapdb.NewMapDB(),
		Identity: identity.GenerateLocalIdentity(),
	}

	for _, option := range options {
		option(builtOptions)
	}

	return
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

// WithoutOpinionFormer an Option for the Tangle that allows to disable the OpinionFormer component.
func WithoutOpinionFormer(with bool) Option {
	return func(options *Options) {
		options.WithoutOpinionFormer = with
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
