package messagelayer

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/gommon/log"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/database"
)

const (
	// DefaultAverageNetworkDelay contains the default average time it takes for a network to propagate through gossip.
	DefaultAverageNetworkDelay = 5 * time.Second
)

// ErrMessageWasNotBookedInTime is returned if a message did not get booked
// within the defined await time.
var ErrMessageWasNotBookedInTime = errors.New("message could not be booked in time")

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	plugin     *node.Plugin
	pluginOnce sync.Once
	syncedOnce sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin("MessageLayer", node.Enabled, configure, run)
	})

	return plugin
}

func configure(plugin *node.Plugin) {
	Tangle().Events.Error.Attach(events.NewClosure(func(err error) {
		plugin.LogError(err)
	}))

	Tangle().Parser.Events.MessageRejected.Attach(events.NewClosure(func(rejectedEvent *tangle.MessageRejectedEvent, err error) {
		plugin.LogError(err)
		plugin.LogError(rejectedEvent.Message)
	}))

	// Messages created by the node need to pass through the normal flow.
	Tangle().RateSetter.Events.MessageIssued.Attach(events.NewClosure(func(message *tangle.Message) {
		Tangle().ProcessGossipMessage(message.Bytes(), local.GetInstance().Peer)
	}))

	Tangle().RateSetter.Events.MessageDiscarded.Attach(events.NewClosure(func(messageID *tangle.MessageID) {
		plugin.LogInfof("issuing queue is full. Message discarded. %s", messageID)
	}))

	Tangle().Scheduler.Events.NodeBlacklisted.Attach(events.NewClosure(func(nodeID identity.ID) {
		// TODO: node blacklisted.
	}))

	Tangle().Events.SyncChanged.Attach(events.NewClosure(func(ev *tangle.SyncChangedEvent) {
		plugin.LogInfo("Sync changed: ", ev.Synced)
		if ev.Synced {
			Tangle().Scheduler.SetRate(schedulerRate(SchedulerParameters.Rate))
			// Only for the first synced
			syncedOnce.Do(func() {
				Tangle().Scheduler.Setup()        // start buffering solid messages
				Tangle().FifoScheduler.Detach()   // stop receiving more messages
				Tangle().FifoScheduler.Shutdown() // schedule remaining messages
				Tangle().Scheduler.Start()        // start scheduler
			})
		} else {
			// increase scheduler rate
			rate := Tangle().Options.SchedulerParams.Rate
			rate -= rate / 2 // 50% increase
			Tangle().Scheduler.SetRate(rate)
		}
	}))

	// read snapshot file
	if Parameters.Snapshot.File != "" {
		snapshot := &ledgerstate.Snapshot{}
		f, err := os.Open(Parameters.Snapshot.File)
		if err != nil {
			plugin.Panic("can not open snapshot file:", err)
		}
		if _, err := snapshot.ReadFrom(f); err != nil {
			plugin.Panic("could not read snapshot file:", err)
		}
		Tangle().LedgerState.LoadSnapshot(snapshot)
		plugin.LogInfof("read snapshot from %s", Parameters.Snapshot.File)
	}

	fcob.LikedThreshold = time.Duration(Parameters.FCOB.AverageNetworkDelay) * time.Second
	fcob.LocallyFinalizedThreshold = time.Duration(Parameters.FCOB.AverageNetworkDelay*2) * time.Second

	configureApprovalWeight()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		Tangle().Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	tangleInstance *tangle.Tangle
	tangleOnce     sync.Once
)

// Tangle gets the tangle instance.
func Tangle() *tangle.Tangle {
	tangleOnce.Do(func() {
		epochManager := epochs.NewManager(epochs.ManaRetriever(ManaEpoch))
		tangleInstance = tangle.New(
			tangle.Store(database.Store()),
			tangle.Identity(local.GetInstance().LocalIdentity()),
			tangle.Width(Parameters.TangleWidth),
			tangle.Consensus(ConsensusMechanism()),
			tangle.GenesisNode(Parameters.Snapshot.GenesisNode),
			tangle.ApprovalWeights(tangle.WeightProviderFromEpochsManager(epochManager)),
			tangle.SchedulerConfig(tangle.SchedulerParams{
				Rate:                        schedulerRate(SchedulerParameters.Rate),
				AccessManaRetrieveFunc:      accessManaRetriever,
				TotalAccessManaRetrieveFunc: totalAccessManaRetriever,
			}),
			tangle.RateSetterConfig(tangle.RateSetterParams{
				Beta:    &RateSetterParameters.Beta,
				Initial: &RateSetterParameters.Initial,
			}),
		)

		tangleInstance.Setup()
	})
	return tangleInstance
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusMechanism ///////////////////////////////////////////////////////////////////////////////////////////

var (
	consensusMechanism     *fcob.ConsensusMechanism
	consensusMechanismOnce sync.Once
)

// ConsensusMechanism return the FcoB ConsensusMechanism used by the Tangle.
func ConsensusMechanism() *fcob.ConsensusMechanism {
	consensusMechanismOnce.Do(func() {
		consensusMechanism = fcob.NewConsensusMechanism()
	})

	return consensusMechanism
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func schedulerRate(durationString string) time.Duration {
	duration, err := time.ParseDuration(durationString)
	// if parseDuration failed, scheduler will take default value (5ms)
	if err != nil {
		return 0
	}
	return duration
}

func accessManaRetriever(nodeID identity.ID) float64 {
	nodeMana, _, err := GetAccessMana(nodeID)
	if err != nil {
		return 0
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
		Tangle().Storage.Message(msgID).Consume(func(message *tangle.Message) {
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
	Tangle().Booker.Events.MessageBooked.Attach(closure)
	defer Tangle().Booker.Events.MessageBooked.Detach(closure)

	// then issue the message with the tx

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
		return nil, xerrors.Errorf("Failed to issue transaction %s: %w", txID.String(), result.err)
	}

	select {
	case <-time.After(maxAwait):
		return nil, ErrMessageWasNotBookedInTime
	case <-booked:
		return result.msg, nil
	}
}
