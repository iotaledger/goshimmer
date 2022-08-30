package tangleold

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

const (
	// DefaultSyncTimeWindow is the default sync time window.
	DefaultSyncTimeWindow = 2 * time.Minute
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	dagMutex *syncutils.DAGMutex[BlockID]

	Options               *Options
	Parser                *Parser
	Storage               *Storage
	Solidifier            *Solidifier
	Scheduler             *Scheduler
	RateSetter            *RateSetter
	Booker                *Booker
	ApprovalWeightManager *ApprovalWeightManager
	TimeManager           *TimeManager
	OTVConsensusManager   *OTVConsensusManager
	TipManager            *TipManager
	Requester             *Requester
	BlockFactory          *BlockFactory
	Ledger                *ledger.Ledger
	Utils                 *Utils
	WeightProvider        WeightProvider
	Events                *Events
	ConfirmationOracle    ConfirmationOracle
	OrphanageManager      *OrphanageManager
}

// ConfirmationOracle answers questions about entities' confirmation.
type ConfirmationOracle interface {
	IsMarkerConfirmed(marker markers.Marker) bool
	IsBlockConfirmed(blkID BlockID) bool
	IsConflictConfirmed(conflictID utxo.TransactionID) bool
	IsTransactionConfirmed(transactionID utxo.TransactionID) bool
	FirstUnconfirmedMarkerIndex(sequenceID markers.SequenceID) (unconfirmedMarkerIndex markers.Index)
	Events() *ConfirmationEvents
}

// New is the constructor for the Tangle.
func New(options ...Option) (tangle *Tangle) {
	tangle = &Tangle{
		dagMutex: syncutils.NewDAGMutex[BlockID](),
		Events:   newEvents(),
	}

	tangle.Configure(options...)

	if !tangle.Options.GenesisTime.IsZero() {
		epoch.GenesisTime = tangle.Options.GenesisTime.Unix()
	}

	tangle.Parser = NewParser()
	tangle.Storage = NewStorage(tangle)
	tangle.Ledger = ledger.New(ledger.WithStore(tangle.Options.Store), ledger.WithVM(new(devnetvm.VM)), ledger.WithCacheTimeProvider(tangle.Options.CacheTimeProvider), ledger.WithConflictDAGOptions(tangle.Options.ConflictDAGOptions...))
	tangle.Solidifier = NewSolidifier(tangle)
	tangle.Scheduler = NewScheduler(tangle)
	tangle.RateSetter = NewRateSetter(tangle)
	tangle.Booker = NewBooker(tangle)
	tangle.OrphanageManager = NewOrphanageManager(tangle)
	tangle.ApprovalWeightManager = NewApprovalWeightManager(tangle)
	tangle.TimeManager = NewTimeManager(tangle)
	tangle.Requester = NewRequester(tangle)
	tangle.TipManager = NewTipManager(tangle)
	tangle.BlockFactory = NewBlockFactory(tangle, tangle.TipManager)
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
}

// Setup sets up the data flow by connecting the different components (by calling their corresponding Setup method).
func (t *Tangle) Setup() {
	t.Parser.Setup()
	t.Storage.Setup()
	t.Solidifier.Setup()
	t.Requester.Setup()
	t.RateSetter.Setup()
	t.Booker.Setup()
	t.OrphanageManager.Setup()
	t.ApprovalWeightManager.Setup()
	t.Scheduler.Setup()
	t.TimeManager.Setup()
	t.TipManager.Setup()

	t.BlockFactory.Events.Error.Attach(event.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in BlockFactory: %w", err))
	}))

	t.Booker.Events.Error.Attach(event.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in booker: %w", err))
	}))

	t.Scheduler.Events.Error.Attach(event.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in Scheduler: %w", err))
	}))

	t.RateSetter.Events.Error.Attach(event.NewClosure(func(err error) {
		t.Events.Error.Trigger(errors.Errorf("error in RateSetter: %w", err))
	}))
}

// ProcessGossipBlock is used to feed new Blocks from the gossip layer into the Tangle.
func (t *Tangle) ProcessGossipBlock(blockBytes []byte, peer *peer.Peer) {
	t.Parser.Parse(blockBytes, peer)
}

// IssuePayload allows to attach a payload (i.e. a Transaction) to the Tangle.
func (t *Tangle) IssuePayload(p payload.Payload, parentsCount ...int) (block *Block, err error) {
	// TODO: after breaking up the tangle package, this needs to use bootstrapmanager.Boostrapped() instead. Currently needs to use this one because the bootstrapmanager is not available here.
	if !t.Bootstrapped() {
		err = errors.Errorf("can't issue payload: %w", ErrNotBootstrapped)
		return
	}
	return t.BlockFactory.IssuePayload(p, parentsCount...)
}

// Bootstrapped returns a boolean value that indicates if the node has bootstrapped and the Tangle has solidified all blocks
// until the genesis.
func (t *Tangle) Bootstrapped() bool {
	return t.TimeManager.Bootstrapped()
}

// Synced returns a boolean value that indicates if the node is in sync at this moment.
func (t *Tangle) Synced() bool {
	return t.TimeManager.Synced()
}

// Prune resets the database and deletes all stored objects (good for testing or "node resets").
func (t *Tangle) Prune() (err error) {
	return t.Storage.Prune()
}

// Shutdown marks the tangle as stopped, so it will not accept any new blocks (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.Requester.Shutdown()
	t.Parser.Shutdown()
	t.BlockFactory.Shutdown()
	t.RateSetter.Shutdown()
	t.Scheduler.Shutdown()
	t.Booker.Shutdown()
	t.ApprovalWeightManager.Shutdown()
	t.Storage.Shutdown()
	t.Ledger.Shutdown()
	t.TimeManager.Shutdown()
	t.TipManager.Shutdown()

	if t.WeightProvider != nil {
		t.WeightProvider.Shutdown()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Tangle to
// configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Tangle.
type Options struct {
	Store                          kvstore.KVStore
	ConflictDAGOptions             []conflictdag.Option
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
	CommitmentFunc                 func() (ecRecord *epoch.ECRecord, lastConfirmedEpochIndex epoch.Index, err error)
	GenesisTime                    time.Time
}

// Store is an Option for the Tangle that allows to specify which storage layer is supposed to be used to persist data.
func Store(store kvstore.KVStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}

// Identity is an Option for the Tangle that allows to specify the node identity which is used to issue Blocks.
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
// to the Genesis Block.
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

// ApprovalWeights is an Option for the Tangle that allows to define how the approval weights of Blocks is determined.
func ApprovalWeights(weightProvider WeightProvider) Option {
	return func(options *Options) {
		options.WeightProvider = weightProvider
	}
}

// GenesisTime is an Option for the Tangle that allows to set the genesis time.
func GenesisTime(genesisTime time.Time) Option {
	return func(options *Options) {
		options.GenesisTime = genesisTime
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

// WithConflictDAGOptions is an Option for the Tangle that allows to set the ConflictDAG options.
func WithConflictDAGOptions(conflictDAGOptions ...conflictdag.Option) Option {
	return func(o *Options) {
		o.ConflictDAGOptions = conflictDAGOptions
	}
}

// CommitmentFunc is an Option for the Tangle that retrieves epoch commitments for blocks.
func CommitmentFunc(commitmentRetrieverFunc func() (*epoch.ECRecord, epoch.Index, error)) Option {
	return func(o *Options) {
		o.CommitmentFunc = commitmentRetrieverFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WeightProvider //////////////////////////////////////////////////////////////////////////////////////////////////////

// WeightProvider is an interface that allows the ApprovalWeightManager to determine approval weights of Blocks
// in a flexible way, independently of a specific implementation.
type WeightProvider interface {
	// Update updates the underlying data structure and keeps track of active nodes.
	Update(ei epoch.Index, nodeID identity.ID)

	// Remove updates the underlying data structure by removing node from active list if no activity left.
	Remove(ei epoch.Index, nodeID identity.ID, decreaseBy uint64) (removed bool)

	// Weight returns the weight and total weight for the given block.
	Weight(block *Block) (weight, totalWeight float64)

	// WeightsOfRelevantVoters returns all relevant weights.
	WeightsOfRelevantVoters() (weights map[identity.ID]float64, totalWeight float64)

	// SnapshotEpochActivity returns the activity log for snapshotting.
	SnapshotEpochActivity(epochDiffIndex epoch.Index) (epochActivity epoch.SnapshotEpochActivity)

	// LoadActiveNodes loads active nodes from the snapshot activity log.
	LoadActiveNodes(loadedActiveNodes epoch.SnapshotEpochActivity)

	// Shutdown shuts down the WeightProvider and persists its state.
	Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
