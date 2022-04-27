package messagelayer

import (
	"context"
	"os"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/consensus/otv"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
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
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle           *tangle.Tangle
	Local            *peer.Local
	Discover         *discover.Protocol `optional:"true"`
	Storage          kvstore.KVStore
	RemoteLoggerConn *remotelog.RemoteLoggerConn `optional:"true"`
}

type tangledeps struct {
	dig.In

	Storage kvstore.KVStore
	Local   *peer.Local
}

func init() {
	Plugin = node.NewPlugin("MessageLayer", deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
		if err := event.Container.Provide(newTangle); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(FinalityGadget); err != nil {
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
	deps.Tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		plugin.LogError(err)
	}))

	// Messages created by the node need to pass through the normal flow.
	deps.Tangle.MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(func(message *tangle.Message) {
		deps.Tangle.ProcessGossipMessage(message.Bytes(), deps.Local.Peer)
	}))

	deps.Tangle.Storage.Events.MessageStored.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			deps.Tangle.WeightProvider.Update(message.IssuingTime(), identity.NewID(message.IssuerPublicKey()))
		})
	}))

	deps.Tangle.Parser.Events.MessageRejected.Attach(events.NewClosure(func(ev *tangle.MessageRejectedEvent, err error) {
		plugin.LogInfof("message with %s rejected in Parser: %v", ev.Message.ID().Base58(), err)
	}))

	deps.Tangle.Parser.Events.BytesRejected.Attach(events.NewClosure(func(ev *tangle.BytesRejectedEvent, err error) {
		if errors.Is(err, tangle.ErrReceivedDuplicateBytes) {
			return
		}

		plugin.LogWarnf("bytes rejected from peer %s: %v", ev.Peer.ID(), err)
	}))

	deps.Tangle.Scheduler.Events.MessageDiscarded.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		plugin.LogInfof("message rejected in Scheduler: %s", messageID.Base58())
	}))

	deps.Tangle.Scheduler.Events.NodeBlacklisted.Attach(events.NewClosure(func(nodeID identity.ID) {
		plugin.LogInfof("node %s is blacklisted in Scheduler", nodeID.String())
	}))

	deps.Tangle.TimeManager.Events.SyncChanged.Attach(events.NewClosure(func(ev *tangle.SyncChangedEvent) {
		plugin.LogInfo("Sync changed: ", ev.Synced)
	}))

	// read snapshot file
	if loaded, _ := deps.Storage.Has(snapshotLoadedKey); !loaded && Parameters.Snapshot.File != "" {
		snapshot := &ledgerstate.Snapshot{}
		f, err := os.Open(Parameters.Snapshot.File)
		if err != nil {
			plugin.Panic("can not open snapshot file:", err)
		}
		plugin.LogInfof("reading snapshot from %s ...", Parameters.Snapshot.File)
		if _, err := snapshot.ReadFrom(f); err != nil {
			plugin.Panic("could not read snapshot file in message layer plugin:", err)
		}
		if err = deps.Tangle.LedgerState.LoadSnapshot(snapshot); err != nil {
			plugin.Panic("fail to load snapshot file in message layer plugin:", err)
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
func newTangle(deps tangledeps) *tangle.Tangle {
	tangleInstance = tangle.New(
		tangle.Store(deps.Storage),
		tangle.Identity(deps.Local.LocalIdentity()),
		tangle.Width(Parameters.TangleWidth),
		tangle.TimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
		tangle.GenesisNode(Parameters.Snapshot.GenesisNode),
		tangle.SchedulerConfig(tangle.SchedulerParams{
			MaxBufferSize:                     SchedulerParameters.MaxBufferSize,
			ConfirmedMessageScheduleThreshold: parseDuration(SchedulerParameters.ConfirmedMessageThreshold),
			Rate:                              parseDuration(SchedulerParameters.Rate),
			AccessManaMapRetrieverFunc:        accessManaMapRetriever,
			AccessManaRetrieveFunc:            accessManaRetriever,
			TotalAccessManaRetrieveFunc:       totalAccessManaRetriever,
		}),
		tangle.RateSetterConfig(tangle.RateSetterParams{
			Initial: &RateSetterParameters.Initial,
		}),
		tangle.SyncTimeWindow(Parameters.TangleTimeWindow),
		tangle.StartSynced(Parameters.StartSynced),
		tangle.CacheTimeProvider(database.CacheTimeProvider()),
	)

	tangleInstance.Scheduler = tangle.NewScheduler(tangleInstance)
	tangleInstance.WeightProvider = tangle.NewCManaWeightProvider(GetCMana, tangleInstance.TimeManager.Time, deps.Storage)
	tangleInstance.OTVConsensusManager = tangle.NewOTVConsensusManager(otv.NewOnTangleVoting(tangleInstance.LedgerState.BranchDAG, tangleInstance.ApprovalWeightManager.WeightOfBranch))

	finalityGadget = finality.NewSimpleFinalityGadget(tangleInstance)
	tangleInstance.ConfirmationOracle = finalityGadget

	tangleInstance.Setup()
	return tangleInstance
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

func accessManaRetriever(nodeID identity.ID) float64 {
	nodeMana, _, err := GetAccessMana(nodeID)
	// return at least MinMana so that zero mana nodes can access the network
	if err != nil && nodeMana < tangle.MinMana {
		return tangle.MinMana
	}
	return nodeMana
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
func AwaitMessageToBeBooked(f func() (*tangle.Message, error), txID ledgerstate.TransactionID, maxAwait time.Duration) (*tangle.Message, error) {
	// first subscribe to the transaction booked event
	booked := make(chan struct{}, 1)
	// exit is used to let the caller exit if for whatever
	// reason the same transaction gets booked multiple times
	exit := make(chan struct{})
	defer close(exit)

	closure := events.NewClosure(func(msgID tangle.MessageID) {
		match := false
		deps.Tangle.Storage.Message(msgID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == ledgerstate.TransactionType {
				tx := message.Payload().(*ledgerstate.Transaction)
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

	closure := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
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
