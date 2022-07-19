package blocklayer

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/lo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/consensus/otv"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/snapshot"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

var (
	// ErrBlockWasNotBookedInTime is returned if a block did not get booked within the defined await time.
	ErrBlockWasNotBookedInTime = errors.New("block could not be booked in time")

	// ErrBlockWasNotIssuedInTime is returned if a block did not get issued within the defined await time.
	ErrBlockWasNotIssuedInTime = errors.New("block could not be issued in time")

	snapshotLoadedKey = kvstore.Key("snapshot_loaded")
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin is the plugin instance of the blocklayer plugin.
	Plugin       *node.Plugin
	deps         = new(dependencies)
	nodeSnapshot *snapshot.Snapshot
)

type dependencies struct {
	dig.In

	Tangle           *tangle.Tangle
	Indexer          *indexer.Indexer
	Local            *peer.Local
	Discover         *discover.Protocol `optional:"true"`
	Storage          kvstore.KVStore
	RemoteLoggerConn *remotelog.RemoteLoggerConn `optional:"true"`
	NotarizationMgr  *notarization.Manager
}

type tangledeps struct {
	dig.In

	Storage kvstore.KVStore
	Local   *peer.Local
}

type indexerdeps struct {
	dig.In

	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("BlockLayer", deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newTangle); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(AcceptanceGadget); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(newIndexer); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("blocklayer")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	deps.Tangle.Events.Error.Attach(event.NewClosure(func(err error) {
		plugin.LogError(err)
	}))

	// Blocks created by the node need to pass through the normal flow.
	deps.Tangle.RateSetter.Events.BlockIssued.Attach(event.NewClosure(func(event *tangle.BlockConstructedEvent) {
		deps.Tangle.ProcessGossipBlock(lo.PanicOnErr(event.Block.Bytes()), deps.Local.Peer)
	}))

	deps.Tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *tangle.BlockBookedEvent) {
		deps.Tangle.Storage.Block(event.BlockID).Consume(func(block *tangle.Block) {
			deps.Tangle.WeightProvider.Update(block.IssuingTime(), identity.NewID(block.IssuerPublicKey()))
		})
	}))

	deps.Tangle.Parser.Events.BlockRejected.Attach(event.NewClosure(func(event *tangle.BlockRejectedEvent) {
		plugin.LogInfof("block with %s rejected in Parser: %v", event.Block.ID().Base58(), event.Error)
	}))

	deps.Tangle.Parser.Events.BytesRejected.Attach(event.NewClosure(func(event *tangle.BytesRejectedEvent) {
		if errors.Is(event.Error, tangle.ErrReceivedDuplicateBytes) {
			return
		}

		plugin.LogWarnf("bytes rejected from peer %s: %v", event.Peer.ID(), event.Error)
	}))

	deps.Tangle.Scheduler.Events.BlockDiscarded.Attach(event.NewClosure(func(event *tangle.BlockDiscardedEvent) {
		plugin.LogInfof("block rejected in Scheduler: %s", event.BlockID.Base58())
	}))

	deps.Tangle.Scheduler.Events.NodeBlacklisted.Attach(event.NewClosure(func(event *tangle.NodeBlacklistedEvent) {
		plugin.LogInfof("node %s is blacklisted in Scheduler", event.NodeID.String())
	}))

	deps.Tangle.TimeManager.Events.SyncChanged.Attach(event.NewClosure(func(event *tangle.SyncChangedEvent) {
		plugin.LogInfo("Sync changed: ", event.Synced)
	}))

	// read snapshot file
	if loaded, _ := deps.Storage.Has(snapshotLoadedKey); !loaded && Parameters.Snapshot.File != "" {
		plugin.LogInfof("reading snapshot from %s ...", Parameters.Snapshot.File)

		nodeSnapshot = new(snapshot.Snapshot)
		err := nodeSnapshot.LoadSnapshot(Parameters.Snapshot.File, deps.Tangle, deps.NotarizationMgr)
		if err != nil {
			plugin.Panic("could not load snapshot file:", err)
		}

		// Add outputs to Indexer.
		for _, outputWithMetadata := range nodeSnapshot.LedgerSnapshot.OutputsWithMetadata {
			deps.Indexer.IndexOutput(outputWithMetadata.Output().(devnetvm.Output))
		}

		for _, epochDiff := range nodeSnapshot.LedgerSnapshot.EpochDiffs {
			for _, outputWithMetadata := range epochDiff.Created() {
				deps.Indexer.IndexOutput(outputWithMetadata.Output().(devnetvm.Output))
			}
		}

		plugin.LogInfof("reading snapshot from %s ... done", Parameters.Snapshot.File)

		// Set flag that we read the snapshot already, so we don't have to do it again after a restart.
		err = deps.Storage.Set(snapshotLoadedKey, kvstore.Value{})
		if err != nil {
			plugin.LogErrorf("could not store snapshot_loaded flag: %v")
		}
	}

	configureFinality()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(ctx context.Context) {
		<-ctx.Done()
		deps.Tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

var tangleInstance *tangle.Tangle

// newTangle gets the tangle instance.
func newTangle(tangleDeps tangledeps) *tangle.Tangle {
	// TODO: this should use the time from the snapshot instead of epoch.GenesisTime
	genesisTime := time.Unix(epoch.GenesisTime, 0)
	if Parameters.GenesisTime > 0 {
		genesisTime = time.Unix(Parameters.GenesisTime, 0)
	}

	tangleInstance = tangle.New(
		tangle.Store(tangleDeps.Storage),
		tangle.Identity(tangleDeps.Local.LocalIdentity()),
		tangle.Width(Parameters.TangleWidth),
		tangle.TimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
		tangle.GenesisNode(Parameters.Snapshot.GenesisNode),
		tangle.SchedulerConfig(tangle.SchedulerParams{
			MaxBufferSize:                   SchedulerParameters.MaxBufferSize,
			TotalSupply:                     2779530283277761,
			ConfirmedBlockScheduleThreshold: parseDuration(SchedulerParameters.ConfirmedBlockThreshold),
			Rate:                            parseDuration(SchedulerParameters.Rate),
			AccessManaMapRetrieverFunc:      accessManaMapRetriever,
			TotalAccessManaRetrieveFunc:     totalAccessManaRetriever,
		}),
		tangle.RateSetterConfig(tangle.RateSetterParams{
			Initial:          RateSetterParameters.Initial,
			RateSettingPause: RateSetterParameters.RateSettingPause,
			Enabled:          RateSetterParameters.Enable,
		}),
		tangle.GenesisTime(genesisTime),
		tangle.SyncTimeWindow(Parameters.TangleTimeWindow),
		tangle.StartSynced(Parameters.StartSynced),
		tangle.CacheTimeProvider(database.CacheTimeProvider()),
		tangle.CommitmentFunc(GetLatestEC),
	)

	tangleInstance.Scheduler = tangle.NewScheduler(tangleInstance)
	tangleInstance.WeightProvider = tangle.NewCManaWeightProvider(GetCMana, tangleInstance.TimeManager.ActivityTime, tangleDeps.Storage)
	tangleInstance.OTVConsensusManager = tangle.NewOTVConsensusManager(otv.NewOnTangleVoting(tangleInstance.Ledger.ConflictDAG, tangleInstance.ApprovalWeightManager.WeightOfConflict))

	acceptanceGadget = acceptance.NewSimpleFinalityGadget(tangleInstance)
	tangleInstance.ConfirmationOracle = acceptanceGadget

	tangleInstance.Setup()
	return tangleInstance
}

func newIndexer(indexerDeps indexerdeps) *indexer.Indexer {
	return indexer.New(indexerDeps.Tangle.Ledger, indexer.WithStore(indexerDeps.Tangle.Options.Store), indexer.WithCacheTimeProvider(database.CacheTimeProvider()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Scheduler ///////////////////////////////////////////////////////////////////////////////////////////

func parseDuration(durationString string) time.Duration {
	duration, err := time.ParseDuration(durationString)
	// if parseDuration failed, scheduler will take default value (5ms)
	if err != nil {
		return 0
	}
	return duration
}

func accessManaMapRetriever() map[identity.ID]float64 {
	nodeMap, _, err := GetManaMap(mana.AccessMana)
	if err != nil {
		return mana.NodeMap{}
	}
	return nodeMap
}

func totalAccessManaRetriever() float64 {
	totalMana, _, err := GetTotalMana(mana.AccessMana)
	if err != nil {
		return 0
	}
	return totalMana
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// AwaitBlockToBeBooked awaits maxAwait for the given block to get booked.
func AwaitBlockToBeBooked(f func() (*tangle.Block, error), txID utxo.TransactionID, maxAwait time.Duration) (*tangle.Block, error) {
	// first subscribe to the transaction booked event
	booked := make(chan struct{}, 1)
	// exit is used to let the caller exit if for whatever
	// reason the same transaction gets booked multiple times
	exit := make(chan struct{})
	defer close(exit)

	closure := event.NewClosure(func(event *tangle.BlockBookedEvent) {
		match := false
		deps.Tangle.Storage.Block(event.BlockID).Consume(func(block *tangle.Block) {
			if block.Payload().Type() == devnetvm.TransactionType {
				tx := block.Payload().(*devnetvm.Transaction)
				if tx.ID() == txID {
					match = true
					return
				}
			}
		})
		if !match {
			return
		}
		select {
		case booked <- struct{}{}:
		case <-exit:
		}
	})
	deps.Tangle.Booker.Events.BlockBooked.Attach(closure)
	defer deps.Tangle.Booker.Events.BlockBooked.Detach(closure)

	// then issue the block with the tx
	blk, err := f()

	if err != nil || blk == nil {
		return nil, errors.Errorf("Failed to issue transaction %s: %w", txID.String(), err)
	}

	select {
	case <-time.After(maxAwait):
		return nil, ErrBlockWasNotBookedInTime
	case <-booked:
		return blk, nil
	}
}

// AwaitBlockToBeIssued awaits maxAwait for the given block to get issued.
func AwaitBlockToBeIssued(f func() (*tangle.Block, error), issuer ed25519.PublicKey, maxAwait time.Duration) (*tangle.Block, error) {
	issued := make(chan *tangle.Block, 1)
	exit := make(chan struct{})
	defer close(exit)

	closure := event.NewClosure(func(event *tangle.BlockScheduledEvent) {
		deps.Tangle.Storage.Block(event.BlockID).Consume(func(block *tangle.Block) {
			if block.IssuerPublicKey() != issuer {
				return
			}
			select {
			case issued <- block:
			case <-exit:
			}
		})
	})
	deps.Tangle.Scheduler.Events.BlockScheduled.Attach(closure)
	defer deps.Tangle.Scheduler.Events.BlockScheduled.Detach(closure)

	// channel to receive the result of issuance
	issueResult := make(chan struct {
		blk *tangle.Block
		err error
	}, 1)

	go func() {
		blk, err := f()
		issueResult <- struct {
			blk *tangle.Block
			err error
		}{blk: blk, err: err}
	}()

	// wait on issuance
	result := <-issueResult

	if result.err != nil || result.blk == nil {
		return nil, errors.Errorf("Failed to issue data: %w", result.err)
	}

	ticker := time.NewTicker(maxAwait)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			return nil, ErrBlockWasNotIssuedInTime
		case blk := <-issued:
			if result.blk.ID() == blk.ID() {
				return blk, nil
			}
		}
	}
}
