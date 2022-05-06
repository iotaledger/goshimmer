package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2021-03-19 9:00:00 UTC.
	DefaultGenesisTime int64 = 1616144400
	// DefaultSyncTimeWindow is the default sync time window.
	DefaultSyncTimeWindow = 2 * time.Minute
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	Options               *Options
	Parser                *Parser
	Storage               *Storage
	Solidifier            *Solidifier
	Scheduler             *Scheduler
	Dispatcher            *Dispatcher
	Booker                *Booker
	ApprovalWeightManager *ApprovalWeightManager
	TimeManager           *TimeManager
	OTVConsensusManager   *OTVConsensusManager
	TipManager            *TipManager
	Requester             *Requester
	MessageFactory        *MessageFactory
	LedgerState           *LedgerState
	Utils                 *Utils
	WeightProvider        WeightProvider
	Events                *Events
	ConfirmationOracle    ConfirmationOracle

	setupParserOnce sync.Once
}

// ConfirmationOracle answers questions about entities' confirmation.
type ConfirmationOracle interface {
	IsMarkerConfirmed(marker *markers.Marker) bool
	IsMessageConfirmed(msgID MessageID) bool
	IsBranchConfirmed(branchID ledgerstate.BranchID) bool
	IsTransactionConfirmed(transactionID ledgerstate.TransactionID) bool
	IsOutputConfirmed(outputID ledgerstate.OutputID) bool
	FirstUnconfirmedMarkerIndex(sequenceID markers.SequenceID) (unconfirmedMarkerIndex markers.Index)
	Events() *ConfirmationEvents
}

// ConfirmationEvents are events entailing confirmation.
type ConfirmationEvents struct {
	MessageConfirmed      *events.Event
	BranchConfirmed       *events.Event
	TransactionConfirmed  *events.Event
	TransactionGoFChanged *events.Event
	BranchGoFChanged      *events.Event
}

// New is the constructor for the Tangle.
func New(options ...Option) (tangle *Tangle) {
	tangle = &Tangle{
		Events: &Events{
			MessageInvalid: events.NewEvent(MessageInvalidCaller),
			Error:          events.NewEvent(events.ErrorCaller),
		},
	}

	tangle.Configure(options...)

	tangle.Parser = NewParser()
	tangle.Storage = NewStorage(tangle)
	tangle.LedgerState = NewLedgerState(tangle)
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Scheduler = NewScheduler(tangle)
	tangle.Booker = NewBooker(tangle)
	tangle.ApprovalWeightManager = NewApprovalWeightManager(tangle)
	tangle.TimeManager = NewTimeManager(tangle)
	tangle.Requester = NewRequester(tangle)
	tangle.TipManager = NewTipManager(tangle)
	tangle.MessageFactory = NewMessageFactory(tangle, tangle.TipManager, PrepareReferences)
	tangle.Utils = NewUtils(tangle)
	tangle.Dispatcher = NewDispatcher(tangle)

	tangle.WeightProvider = tangle.Options.WeightProvider

	return
}

// Configure modifies the configuration of the Tangle.
func (t *Tangle) Configure(options ...Option) {
	if t.Options == nil {
		t.Options = &Options{
			Store:                        mapdb.NewMapDB(),
			Identity:                     identity.GenerateLocalIdentity(),
			IncreaseMarkersIndexCallback: increaseMarkersIndexCallbackStrategy,
			LedgerState:                  struct{ MergeBranches bool }{MergeBranches: true},
		}
	}

	for _, option := range options {
		option(t.Options)
	}
}

// Setup sets up the data flow by connecting the different components (by calling their corresponding Setup method).
func (t *Tangle) Setup() {
	t.Storage.Setup()
	t.Solidifier.Setup()
	t.Requester.Setup()
	t.Scheduler.Setup()
	t.Dispatcher.Setup()
	t.Booker.Setup()
	t.LedgerState.Setup()
	t.ApprovalWeightManager.Setup()
	t.TimeManager.Setup()
	t.TipManager.Setup()

	t.MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in MessageFactory: %w", err))
	}))

	t.Booker.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in Booker: %w", err))
	}))

	t.Scheduler.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in Scheduler: %w", err))
	}))
}

// ProcessGossipMessage is used to feed new Messages from the gossip layer into the Tangle.
func (t *Tangle) ProcessGossipMessage(messageBytes []byte, peer *peer.Peer) {
	t.setupParserOnce.Do(t.Parser.Setup)
	t.Parser.Parse(messageBytes, peer)
}

// IssuePayload allows to attach a payload (i.e. a Transaction) to the Tangle.
func (t *Tangle) IssuePayload(p payload.Payload, parentsCount ...int) (message *Message, err error) {
	if !t.Synced() {
		err = errors.Errorf("can't issue payload: %w", ErrNotSynced)
		return
	}

	return t.MessageFactory.IssuePayload(p, parentsCount...)
}

// Synced returns a boolean value that indicates if the node is fully synced and the Tangle has solidified all messages
// until the genesis.
func (t *Tangle) Synced() (synced bool) {
	return t.TimeManager.Synced()
}

// Prune resets the database and deletes all stored objects (good for testing or "node resets").
func (t *Tangle) Prune() (err error) {
	return t.Storage.Prune()
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.Requester.Shutdown()
	t.Parser.Shutdown()
	t.MessageFactory.Shutdown()
	t.Scheduler.Shutdown()
	t.Dispatcher.Shutdown()
	t.Booker.Shutdown()
	t.ApprovalWeightManager.Shutdown()
	t.Storage.Shutdown()
	t.LedgerState.Shutdown()
	t.TimeManager.Shutdown()
	t.TipManager.Shutdown()

	if t.WeightProvider != nil {
		t.WeightProvider.Shutdown()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Tangle.
type Events struct {
	// MessageInvalid is triggered when a Message is detected to be objectively invalid.
	MessageInvalid *events.Event

	// Error is triggered when the Tangle faces an error from which it can not recover.
	Error *events.Event
}

// MessageIDCaller is the caller function for events that hand over a MessageID.
func MessageIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(MessageID))(params[0].(MessageID))
}

// MessageCaller is the caller function for events that hand over a Message.
func MessageCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Message))(params[0].(*Message))
}

// MessageInvalidCaller is the caller function for events that had over an invalid message.
func MessageInvalidCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *MessageInvalidEvent))(params[0].(*MessageInvalidEvent))
}

// MessageInvalidEvent is struct that is passed along with triggering a messageInvalidEvent.
type MessageInvalidEvent struct {
	MessageID MessageID
	Error     error
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Tangle to
// configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Tangle.
type Options struct {
	Store                          kvstore.KVStore
	Identity                       *identity.LocalIdentity
	IncreaseMarkersIndexCallback   markers.IncreaseIndexCallback
	TangleWidth                    int
	GenesisNode                    *ed25519.PublicKey
	SchedulerParams                SchedulerParams
	RateSetterParams               RateSetterParams
	WeightProvider                 WeightProvider
	SyncTimeWindow                 time.Duration
	TimeSinceConfirmationThreshold time.Duration
	StartSynced                    bool
	CacheTimeProvider              *database.CacheTimeProvider
	LedgerState                    struct{ MergeBranches bool }
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

// IncreaseMarkersIndexCallback is an Option for the Tangle that allows to change the strategy how new Markers are
// assigned in the Tangle.
func IncreaseMarkersIndexCallback(callback markers.IncreaseIndexCallback) Option {
	return func(options *Options) {
		options.IncreaseMarkersIndexCallback = callback
	}
}

// Width is an Option for the Tangle that allows to change the strategy how Tips get removed.
func Width(width int) Option {
	return func(options *Options) {
		options.TangleWidth = width
	}
}

// TimeSinceConfirmationThreshold is an Option for the Tangle that allows to set threshold for Time Since Confirmation check.
func TimeSinceConfirmationThreshold(tscThreshold time.Duration) Option {
	return func(options *Options) {
		options.TimeSinceConfirmationThreshold = tscThreshold
	}
}

// GenesisNode is an Option for the Tangle that allows to set the GenesisNode, i.e., the node that is allowed to attach
// to the Genesis Message.
func GenesisNode(genesisNodeBase58 string) Option {
	var genesisPublicKey *ed25519.PublicKey
	pkBytes, _ := base58.Decode(genesisNodeBase58)
	pk, _, err := ed25519.PublicKeyFromBytes(pkBytes)
	if err == nil {
		genesisPublicKey = &pk
	}

	return func(options *Options) {
		options.GenesisNode = genesisPublicKey
	}
}

// SchedulerConfig is an Option for the Tangle that allows to set the scheduler.
func SchedulerConfig(config SchedulerParams) Option {
	return func(options *Options) {
		options.SchedulerParams = config
	}
}

// RateSetterConfig is an Option for the Tangle that allows to set the rate setter.
func RateSetterConfig(params RateSetterParams) Option {
	return func(options *Options) {
		options.RateSetterParams = params
	}
}

// ApprovalWeights is an Option for the Tangle that allows to define how the approval weights of Messages is determined.
func ApprovalWeights(weightProvider WeightProvider) Option {
	return func(options *Options) {
		options.WeightProvider = weightProvider
	}
}

// SyncTimeWindow is an Option for the Tangle that allows to define the time window in which the node will consider
// itself in sync.
func SyncTimeWindow(syncTimeWindow time.Duration) Option {
	return func(options *Options) {
		options.SyncTimeWindow = syncTimeWindow
	}
}

// StartSynced is an Option for the Tangle that allows to define if the node starts as synced.
func StartSynced(startSynced bool) Option {
	return func(options *Options) {
		options.StartSynced = startSynced
	}
}

// CacheTimeProvider is an Option for the Tangle that allows to override hard coded cache time.
func CacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *Options) {
		options.CacheTimeProvider = cacheTimeProvider
	}
}

// MergeBranches is an Option for the Tangle that prevents the LedgerState from merging Branches.
func MergeBranches(mergeBranches bool) Option {
	return func(o *Options) {
		o.LedgerState.MergeBranches = mergeBranches
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WeightProvider //////////////////////////////////////////////////////////////////////////////////////////////////////

// WeightProvider is an interface that allows the ApprovalWeightManager to determine approval weights of Messages
// in a flexible way, independently of a specific implementation.
type WeightProvider interface {
	// Update updates the underlying data structure and keeps track of active nodes.
	Update(t time.Time, nodeID identity.ID)

	// Weight returns the weight and total weight for the given message.
	Weight(message *Message) (weight, totalWeight float64)

	// WeightsOfRelevantVoters returns all relevant weights.
	WeightsOfRelevantVoters() (weights map[identity.ID]float64, totalWeight float64)

	// Shutdown shuts down the WeightProvider and persists its state.
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
