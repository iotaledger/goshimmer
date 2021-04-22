package tangle

import (
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	Options               *Options
	Parser                *Parser
	Storage               *Storage
	Solidifier            *Solidifier
	Scheduler             *Scheduler
	Booker                *Booker
	ApprovalWeightManager *ApprovalWeightManager
	ConsensusManager      *ConsensusManager
	TipManager            *TipManager
	Requester             *Requester
	MessageFactory        *MessageFactory
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
			Error:           events.NewEvent(events.ErrorCaller),
		},
	}

	tangle.Configure(options...)

	tangle.Parser = NewParser()
	tangle.Storage = NewStorage(tangle)
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Scheduler = NewScheduler(tangle)
	tangle.LedgerState = NewLedgerState(tangle)
	tangle.Booker = NewBooker(tangle)
	tangle.ApprovalWeightManager = NewApprovalWeightManager(tangle)
	tangle.ConsensusManager = NewConsensusManager(tangle)
	tangle.Requester = NewRequester(tangle)
	tangle.TipManager = NewTipManager(tangle)
	tangle.MessageFactory = NewMessageFactory(tangle, tangle.TipManager)
	tangle.Utils = NewUtils(tangle)

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
	t.Scheduler.Setup()
	t.Booker.Setup()
	t.ApprovalWeightManager.Setup()
	t.ConsensusManager.Setup()
	t.TipManager.Setup()

	t.MessageFactory.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(xerrors.Errorf("error in MessageFactory: %w", err))
	}))

	t.Booker.Events.Error.Attach(events.NewClosure(func(err error) {
		t.Events.Error.Trigger(xerrors.Errorf("error in Booker: %w", err))
	}))
}

// ProcessGossipMessage is used to feed new Messages from the gossip layer into the Tangle.
func (t *Tangle) ProcessGossipMessage(messageBytes []byte, peer *peer.Peer) {
	t.setupParserOnce.Do(t.Parser.Setup)

	t.Parser.Parse(messageBytes, peer)
}

// IssuePayload allows to attach a payload (i.e. a Transaction) to the Tangle.
func (t *Tangle) IssuePayload(payload payload.Payload) (message *Message, err error) {
	if !t.Synced() {
		err = xerrors.Errorf("can't issue payload: %w", ErrNotSynced)
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
			return nil, xerrors.Errorf("invalid inputs: %s: %w", strings.Join(invalidInputs, ","), ErrInvalidInputs)
		}
	}

	return t.MessageFactory.IssuePayload(payload)
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

	return
}

// Prune resets the database and deletes all stored objects (good for testing or "node resets").
func (t *Tangle) Prune() (err error) {
	return t.Storage.Prune()
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.MessageFactory.Shutdown()
	t.Scheduler.Shutdown()
	t.Booker.Shutdown()
	t.LedgerState.Shutdown()
	t.ConsensusManager.Shutdown()
	t.Storage.Shutdown()
	t.LedgerState.Shutdown()
	t.Options.Store.Shutdown()
	t.TipManager.Shutdown()
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

// MessageIDCaller is the caller function for events that hand over a MessageID.
func MessageIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(MessageID))(params[0].(MessageID))
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
	WeightProvider               WeightProvider
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

// ApprovalWeights is an Option for the Tangle that allows to define how the approval weights of Messages is determined.
func ApprovalWeights(weightProvider WeightProvider) Option {
	return func(options *Options) {
		options.WeightProvider = weightProvider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WeightProvider //////////////////////////////////////////////////////////////////////////////////////////////////////

// WeightProvider is an interface that allows the ApprovalWeightManager to determine approval weights of Messages
// in a flexible way, independently of a specific implementation.
type WeightProvider interface {
	// Epoch returns the Epoch from the given referenceTime.
	Epoch(referenceTime time.Time) Epoch

	// Weight returns the weight and total weight for the given epoch and message.
	Weight(epoch Epoch, message *Message) (weight, totalWeight float64)

	// WeightsOfRelevantSupporters returns all relevant weights for the given epoch.
	WeightsOfRelevantSupporters(epoch Epoch) (weights map[identity.ID]float64, totalWeight float64)
}

// WeightProviderFromEpochsManager returns a WeightProvider from an epochs.Manager instance so that it can be used as a
// WeightProvider.
func WeightProviderFromEpochsManager(epochManager *epochs.Manager) WeightProvider {
	return &epochsManagerWeightProvider{Manager: epochManager}
}

type epochsManagerWeightProvider struct {
	*epochs.Manager
}

func (e *epochsManagerWeightProvider) Epoch(referenceTime time.Time) Epoch {
	return uint64(e.Manager.TimeToOracleEpochID(referenceTime))
}

func (e *epochsManagerWeightProvider) Weight(_ Epoch, message *Message) (weight, totalWeight float64) {
	weight, totalWeight, _ = e.Manager.RelativeNodeMana(identity.NewID(message.IssuerPublicKey()), message.IssuingTime())

	return weight, totalWeight
}

func (e *epochsManagerWeightProvider) WeightsOfRelevantSupporters(epoch Epoch) (weights map[identity.ID]float64, totalWeight float64) {
	return e.Manager.ActiveMana(epochs.ID(epoch))
}

var _ WeightProvider = &epochsManagerWeightProvider{}

// Epoch is an alias for a uint64 that represents a universal time interval.
type Epoch = uint64

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
