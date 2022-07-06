package messagelayer

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

	"github.com/iotaledger/goshimmer/packages/consensus/finality"
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
	// ErrMessageWasNotBookedInTime is returned if a message did not get booked within the defined await time.
	ErrMessageWasNotBookedInTime = errors.New("message could not be booked in time")

	// ErrMessageWasNotIssuedInTime is returned if a message did not get issued within the defined await time.
	ErrMessageWasNotIssuedInTime = errors.New("message could not be issued in time")

	snapshotLoadedKey = kvstore.Key("snapshot_loaded")
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin is the plugin instance of the messagelayer plugin.
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
	Plugin = node.NewPlugin("MessageLayer", deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newTangle); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(FinalityGadget); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(newIndexer); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("messagelayer")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	deps.Tangle.Events.Error.Attach(event.NewClosure(func(err error) {
		plugin.LogError(err)
	}))

	// Messages created by the node need to pass through the normal flow.
	deps.Tangle.RateSetter.Events.MessageIssued.Attach(event.NewClosure(func(event *tangle.MessageConstructedEvent) {
		deps.Tangle.ProcessGossipMessage(lo.PanicOnErr(event.Message.Bytes()), deps.Local.Peer)
	}))

	deps.Tangle.Storage.Events.MessageStored.Attach(event.NewClosure(func(event *tangle.MessageStoredEvent) {
		deps.Tangle.WeightProvider.Update(event.Message.IssuingTime(), identity.NewID(event.Message.IssuerPublicKey()))
	}))

	deps.Tangle.Parser.Events.MessageRejected.Attach(event.NewClosure(func(event *tangle.MessageRejectedEvent) {
		plugin.LogInfof("message with %s rejected in Parser: %v", event.Message.ID().Base58(), event.Error)
	}))

	deps.Tangle.Parser.Events.BytesRejected.Attach(event.NewClosure(func(event *tangle.BytesRejectedEvent) {
		if errors.Is(event.Error, tangle.ErrReceivedDuplicateBytes) {
			return
		}

		plugin.LogWarnf("bytes rejected from peer %s: %v", event.Peer.ID(), event.Error)
	}))

	deps.Tangle.Scheduler.Events.MessageDiscarded.Attach(event.NewClosure(func(event *tangle.MessageDiscardedEvent) {
		plugin.LogInfof("message rejected in Scheduler: %s", event.MessageID.Base58())
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
		err := nodeSnapshot.FromFile(Parameters.Snapshot.File)
		if err != nil {
			plugin.Panic("could not load snapshot file:", err)
		}

		deps.Tangle.Ledger.LoadSnapshot(nodeSnapshot.LedgerSnapshot)

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

	if Parameters.GenesisTime > 0 {
		epoch.GenesisTime = Parameters.GenesisTime
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
	tangleInstance = tangle.New(
		tangle.Store(tangleDeps.Storage),
		tangle.Identity(tangleDeps.Local.LocalIdentity()),
		tangle.Width(Parameters.TangleWidth),
		tangle.TimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
		tangle.GenesisNode(Parameters.Snapshot.GenesisNode),
		tangle.SchedulerConfig(tangle.SchedulerParams{
			MaxBufferSize:                     SchedulerParameters.MaxBufferSize,
			TotalSupply:                       2779530283277761,
			ConfirmedMessageScheduleThreshold: parseDuration(SchedulerParameters.ConfirmedMessageThreshold),
			Rate:                              parseDuration(SchedulerParameters.Rate),
			AccessManaMapRetrieverFunc:        accessManaMapRetriever,
			TotalAccessManaRetrieveFunc:       totalAccessManaRetriever,
		}),
		tangle.RateSetterConfig(tangle.RateSetterParams{
			Initial:          RateSetterParameters.Initial,
			RateSettingPause: RateSetterParameters.RateSettingPause,
			Enabled:          RateSetterParameters.Enable,
		}),
		tangle.SyncTimeWindow(Parameters.TangleTimeWindow),
		tangle.StartSynced(Parameters.StartSynced),
		tangle.CacheTimeProvider(database.CacheTimeProvider()),
		tangle.CommitmentFunc(GetLatestEC),
	)

	tangleInstance.Scheduler = tangle.NewScheduler(tangleInstance)
	tangleInstance.WeightProvider = tangle.NewCManaWeightProvider(GetCMana, tangleInstance.TimeManager.RATT, tangleDeps.Storage)
	tangleInstance.OTVConsensusManager = tangle.NewOTVConsensusManager(otv.NewOnTangleVoting(tangleInstance.Ledger.ConflictDAG, tangleInstance.ApprovalWeightManager.WeightOfBranch))

	finalityGadget = finality.NewSimpleFinalityGadget(tangleInstance)
	tangleInstance.ConfirmationOracle = finalityGadget

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

// AwaitMessageToBeBooked awaits maxAwait for the given message to get booked.
func AwaitMessageToBeBooked(f func() (*tangle.Message, error), txID utxo.TransactionID, maxAwait time.Duration) (*tangle.Message, error) {
	// first subscribe to the transaction booked event
	booked := make(chan struct{}, 1)
	// exit is used to let the caller exit if for whatever
	// reason the same transaction gets booked multiple times
	exit := make(chan struct{})
	defer close(exit)

	closure := event.NewClosure(func(event *tangle.MessageBookedEvent) {
		match := false
		deps.Tangle.Storage.Message(event.MessageID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == devnetvm.TransactionType {
				tx := message.Payload().(*devnetvm.Transaction)
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
	deps.Tangle.Booker.Events.MessageBooked.Attach(closure)
	defer deps.Tangle.Booker.Events.MessageBooked.Detach(closure)

	// then issue the message with the tx
	msg, err := f()

	if err != nil || msg == nil {
		return nil, errors.Errorf("Failed to issue transaction %s: %w", txID.String(), err)
	}

	select {
	case <-time.After(maxAwait):
		return nil, ErrMessageWasNotBookedInTime
	case <-booked:
		return msg, nil
	}
}

// AwaitMessageToBeIssued awaits maxAwait for the given message to get issued.
func AwaitMessageToBeIssued(f func() (*tangle.Message, error), issuer ed25519.PublicKey, maxAwait time.Duration) (*tangle.Message, error) {
	issued := make(chan *tangle.Message, 1)
	exit := make(chan struct{})
	defer close(exit)

	closure := event.NewClosure(func(event *tangle.MessageScheduledEvent) {
		deps.Tangle.Storage.Message(event.MessageID).Consume(func(message *tangle.Message) {
			if message.IssuerPublicKey() != issuer {
				return
			}
			select {
			case issued <- message:
			case <-exit:
			}
		})
	})
	deps.Tangle.Scheduler.Events.MessageScheduled.Attach(closure)
	defer deps.Tangle.Scheduler.Events.MessageScheduled.Detach(closure)

	// channel to receive the result of issuance
	issueResult := make(chan struct {
		msg *tangle.Message
		err error
	}, 1)

	go func() {
		msg, err := f()
		issueResult <- struct {
			msg *tangle.Message
			err error
		}{msg: msg, err: err}
	}()

	// wait on issuance
	result := <-issueResult

	if result.err != nil || result.msg == nil {
		return nil, errors.Errorf("Failed to issue data: %w", result.err)
	}

	ticker := time.NewTicker(maxAwait)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			return nil, ErrMessageWasNotIssuedInTime
		case msg := <-issued:
			if result.msg.ID() == msg.ID() {
				return msg, nil
			}
		}
	}
}
