package tangle

import (
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2021-03-19 9:00:00 UTC.
	DefaultGenesisTime int64 = 1616144400
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	Options               *Options
	Parser                *Parser
	Storage               *Storage
	Solidifier            *Solidifier
	Scheduler             *Scheduler
	FifoScheduler         *FifoScheduler
	Orderer               *Orderer
	Booker                *Booker
	ApprovalWeightManager *ApprovalWeightManager
	TimeManager           *TimeManager
	ConsensusManager      *ConsensusManager
	TipManager            *TipManager
	Requester             *Requester
	MessageFactory        *MessageFactory
	RateSetter            *RateSetter
	LedgerState           *LedgerState
	Utils                 *Utils
	WeightProvider        WeightProvider
	Events                *Events

	setupParserOnce sync.Once
	syncedMutex     sync.RWMutex
	synced          bool
}

// New is the constructor for the Tangle.
func New(options ...Option) (tangle *Tangle) {
	tangle = &Tangle{
		Events: &Events{
			MessageEligible: events.NewEvent(MessageIDCaller),
			MessageInvalid:  events.NewEvent(MessageIDCaller),
			SyncChanged:     events.NewEvent(SyncChangedCaller),
			Error:           events.NewEvent(events.ErrorCaller),
			Info:            events.NewEvent(events.StringCaller),
		},
	}

	tangle.Configure(options...)

	tangle.Parser = NewParser()
	tangle.Storage = NewStorage(tangle)
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Scheduler = NewScheduler(tangle)
	tangle.FifoScheduler = NewFifoScheduler(tangle)
	tangle.LedgerState = NewLedgerState(tangle)
	tangle.Booker = NewBooker(tangle)
	tangle.ApprovalWeightManager = NewApprovalWeightManager(tangle)
	tangle.TimeManager = NewTimeManager(tangle)
	tangle.ConsensusManager = NewConsensusManager(tangle)
	tangle.Requester = NewRequester(tangle)
	tangle.TipManager = NewTipManager(tangle)
	tangle.MessageFactory = NewMessageFactory(tangle, tangle.TipManager)
	tangle.Utils = NewUtils(tangle)
	tangle.RateSetter = NewRateSetter(tangle)
	tangle.Orderer = NewOrderer(tangle)

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
		}
	}

	for _, option := range options {
		option(t.Options)
	}

	if t.Options.ConsensusMechanism != nil {
		t.Options.ConsensusMechanism.Init(t)
	}
}

// Setup sets up the data flow by connecting the different components (by calling their corresponding Setup method).
func (t *Tangle) Setup() {
	t.Storage.Setup()
	t.Solidifier.Setup()
	t.Requester.Setup()
	t.FifoScheduler.Setup()
	t.Orderer.Setup()
	t.Booker.Setup()
	t.ApprovalWeightManager.Setup()
	t.TimeManager.Setup()
	t.ConsensusManager.Setup()
	t.TipManager.Setup()
	t.RateSetter.Setup()

	t.MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in MessageFactory: %w", err))
	}))

	t.Booker.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in Booker: %w", err))
	}))
}

// ProcessGossipMessage is used to feed new Messages from the gossip layer into the Tangle.
func (t *Tangle) ProcessGossipMessage(messageBytes []byte, peer *peer.Peer) {
	_, _, _ = MessageFromBytes(messageBytes)
	t.setupParserOnce.Do(t.Parser.Setup)
	t.Parser.Parse(messageBytes, peer)
}

// IssuePayload allows to attach a payload (i.e. a Transaction) to the Tangle.
func (t *Tangle) IssuePayload(payload payload.Payload) (message *Message, err error) {
	if !t.Synced() {
		err = errors.Errorf("can't issue payload: %w", ErrNotSynced)
		return
	}

	if payload.Type() == ledgerstate.TransactionType {
		var invalidInputs []string
		transaction := payload.(*ledgerstate.Transaction)
		for _, input := range transaction.Essence().Inputs() {
			if input.Type() == ledgerstate.UTXOInputType {
				t.LedgerState.OutputMetadata(input.(*ledgerstate.UTXOInput).ReferencedOutputID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
					t.LedgerState.BranchDAG.Branch(outputMetadata.BranchID()).Consume(func(branch ledgerstate.Branch) {
						if branch.InclusionState() == ledgerstate.Rejected || !branch.MonotonicallyLiked() {
							invalidInputs = append(invalidInputs, input.Base58())
						}
					})
				})
			}
		}
		if len(invalidInputs) > 0 {
			return nil, errors.Errorf("invalid inputs: %s: %w", strings.Join(invalidInputs, ","), ErrInvalidInputs)
		}
	}

	return t.MessageFactory.IssuePayload(payload, t)
}

// Synced returns a boolean value that indicates if the node is fully synced and the Tangle has solidified all messages
// until the genesis.
func (t *Tangle) Synced() (synced bool) {
	t.syncedMutex.RLock()
	defer t.syncedMutex.RUnlock()

	return t.synced
}

// SetSynced allows to set a boolean value that indicates if the Tangle has solidified all messages until the genesis.
func (t *Tangle) SetSynced(synced bool) (modified bool) {
	t.syncedMutex.Lock()
	defer t.syncedMutex.Unlock()

	if t.synced == synced {
		return
	}

	t.synced = synced
	modified = true

	go func() {
		t.Events.SyncChanged.Trigger(&SyncChangedEvent{synced})
	}()
	return
}

// Prune resets the database and deletes all stored objects (good for testing or "node resets").
func (t *Tangle) Prune() (err error) {
	return t.Storage.Prune()
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.MessageFactory.Shutdown()
	t.RateSetter.Shutdown()
	t.Scheduler.Shutdown()
	t.Orderer.Shutdown()
	t.Booker.Shutdown()
	t.LedgerState.Shutdown()
	t.ConsensusManager.Shutdown()
	t.ApprovalWeightManager.Shutdown()
	t.Storage.Shutdown()
	t.LedgerState.Shutdown()
	t.TimeManager.Shutdown()
	t.Options.Store.Shutdown()
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

	// Fired when a message has been eligible.
	MessageEligible *events.Event

	// Fired when the nodes sync status changes.
	SyncChanged *events.Event

	// Error is triggered when the Tangle faces an error from which it can not recover.
	Error *events.Event

	// Info is triggered when the tangle should log the info provided.
	Info *events.Event
}

// SyncChangedEvent represents a syn changed event.
type SyncChangedEvent struct {
	Synced bool
}

// MessageIDCaller is the caller function for events that hand over a MessageID.
func MessageIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(MessageID))(params[0].(MessageID))
}

// SyncChangedCaller is the caller function for sync changed event.
func SyncChangedCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *SyncChangedEvent))(params[0].(*SyncChangedEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Tangle to
// configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Tangle.
type Options struct {
	Store                        kvstore.KVStore
	Identity                     *identity.LocalIdentity
	IncreaseMarkersIndexCallback markers.IncreaseIndexCallback
	TangleWidth                  int
	ConsensusMechanism           ConsensusMechanism
	GenesisNode                  *ed25519.PublicKey
	SchedulerParams              SchedulerParams
	RateSetterParams             RateSetterParams
	WeightProvider               WeightProvider
	SyncTimeWindow               time.Duration
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

// Consensus is an Option for the Tangle that allows to define the consensus mechanism that is used by the Tangle.
func Consensus(consensusMechanism ConsensusMechanism) Option {
	return func(options *Options) {
		options.ConsensusMechanism = consensusMechanism
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WeightProvider //////////////////////////////////////////////////////////////////////////////////////////////////////

// WeightProvider is an interface that allows the ApprovalWeightManager to determine approval weights of Messages
// in a flexible way, independently of a specific implementation.
type WeightProvider interface {
	// Update updates the underlying data structure and keeps track of active nodes.
	Update(t time.Time, nodeID identity.ID)

	// Weight returns the weight and total weight for the given message.
	Weight(message *Message) (weight, totalWeight float64)

	// WeightsOfRelevantSupporters returns all relevant weights.
	WeightsOfRelevantSupporters() (weights map[identity.ID]float64, totalWeight float64)

	// Shutdown shuts down the WeightProvider and persists its state.
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
