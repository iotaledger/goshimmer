package sync

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/types"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"go.uber.org/atomic"
)

const (
	// PluginName is the plugin name of the sync plugin.
	PluginName = "Sync"
	// CfgSyncAnchorPointsCount defines the amount of anchor points to use to determine
	// whether a node is synchronized.
	CfgSyncAnchorPointsCount = "sync.anchorPointsCount"
	// CfgSyncAnchorPointsCleanupAfterSec defines the amount of time which is allowed to pass between setting an anchor
	// point and it not becoming solid (to clean its slot for another anchor point). It basically defines the expectancy
	// of how long it should take for an anchor point to become solid. Even if this value is set too low, usually a node
	// would eventually solidify collected anchor points.
	CfgSyncAnchorPointsCleanupAfterSec = "sync.anchorPointsCleanupAfterSec"
	// CfgSyncAnchorPointsCleanupIntervalSec defines the interval at which it is checked whether anchor points fall
	// into the cleanup window.
	CfgSyncAnchorPointsCleanupIntervalSec = "sync.anchorPointsCleanupIntervalSec"
	// CfgSyncDesyncedIfNoMessageAfterSec defines the time period in which new messages must be received and if not
	// the node is marked as desynced.
	CfgSyncDesyncedIfNoMessageAfterSec = "sync.desyncedIfNoMessagesAfterSec"
)

func init() {
	flag.Int(CfgSyncAnchorPointsCount, 3, "the amount of anchor points to use to determine whether a node is synchronized")
	flag.Int(CfgSyncDesyncedIfNoMessageAfterSec, 300, "the time period in seconds which sets the node as desynced if no new messages are received")
	flag.Int(CfgSyncAnchorPointsCleanupIntervalSec, 10, "the interval at which it is checked whether anchor points fall into the cleanup window")
	flag.Int(CfgSyncAnchorPointsCleanupAfterSec, 60, "the amount of time which is allowed to pass between setting an anchor point and it not becoming solid (to clean its slot for another anchor point)")
}

var (
	// Plugin is the plugin instance of the sync plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	// ErrNodeNotSynchronized is returned when an operation can't be executed because
	// the node is not synchronized.
	ErrNodeNotSynchronized = errors.New("node is not synchronized")
	// tells whether the node is synced or not.
	synced atomic.Bool
	log    *logger.Logger
)

// Synced tells whether the node is in a state we consider synchronized, meaning
// it has the relevant past and present message data.
func Synced() bool {
	return synced.Load()
}

// OverwriteSyncedState overwrites the synced state with the given value.
func OverwriteSyncedState(syncedOverwrite bool) {
	synced.Store(syncedOverwrite)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	// per default the node starts in a desynced state
	if !Synced() {
		monitorForSynchronization()
		return
	}

	// however, another plugin might want to overwrite the synced state (i.e. the bootstrap plugin)
	// in order to start issuing messages
	monitorForDesynchronization()
}

// marks the node as synced and spawns the background worker to monitor desynchronization.
func markSynced() {
	synced.Store(true)
	monitorForDesynchronization()
}

// marks the node as desynced and spawns the background worker to monitor synchronization.
func markDesynced() {
	synced.Store(false)
	monitorForSynchronization()
}

// starts a background worker and event handlers to check whether the node is desynchronized by checking
// whether the node has no more peers or didn't receive any message in a given time period.
func monitorForDesynchronization() {
	log.Info("monitoring for desynchronization")

	// monitors the peer count of the manager and sets the node as desynced if it has no more peers.
	noPeers := make(chan types.Empty)
	monitorPeerCountClosure := events.NewClosure(func(_ *peer.Peer) {
		anyPeers := len(gossip.Manager().AllNeighbors()) > 0
		if anyPeers {
			return
		}
		noPeers <- types.Empty{}
	})

	msgReceived := make(chan types.Empty, 1)

	monitorMessageInflowClosure := events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		defer cachedMessage.Release()
		defer cachedMessageMetadata.Release()
		// ignore messages sent by the node itself
		if local.GetInstance().LocalIdentity().PublicKey() == cachedMessage.Unwrap().IssuerPublicKey() {
			return
		}
		select {
		case msgReceived <- types.Empty{}:
		default:
			// via this default clause, a slow desync-monitor select-loop
			// worker should not increase latency as it auto. falls through
		}
	})

	daemon.BackgroundWorker("Desync-Monitor", func(shutdownSignal <-chan struct{}) {
		gossip.Manager().Events().NeighborRemoved.Attach(monitorPeerCountClosure)
		defer gossip.Manager().Events().NeighborRemoved.Detach(monitorPeerCountClosure)
		messagelayer.Tangle.Events.MessageAttached.Attach(monitorMessageInflowClosure)
		defer messagelayer.Tangle.Events.MessageAttached.Detach(monitorMessageInflowClosure)

		desyncedIfNoMessageInSec := config.Node.GetDuration(CfgSyncDesyncedIfNoMessageAfterSec) * time.Second
		timer := time.NewTimer(desyncedIfNoMessageInSec)
		for {
			select {

			case <-msgReceived:
				// we received a message, therefore reset the timer to check for message receives
				if !timer.Stop() {
					<-timer.C
				}
				// TODO: perhaps find a better way instead of constantly resetting the timer
				timer.Reset(desyncedIfNoMessageInSec)

			case <-timer.C:
				log.Infof("no message received in %d seconds, marking node as desynced", desyncedIfNoMessageInSec)
				markDesynced()
				return

			case <-noPeers:
				log.Info("all peers have been lost, marking node as desynced")
				markDesynced()
				return

			case <-shutdownSignal:
				return
			}
		}
	}, shutdown.PrioritySynchronization)
}

// starts a background worker and event handlers to check whether the node is synchronized by first collecting
// a set of newly received messages and then waiting for them to become solid.
func monitorForSynchronization() {
	wantedAnchorPointsCount := config.Node.GetInt(CfgSyncAnchorPointsCount)
	anchorPoints := newAnchorPoints(wantedAnchorPointsCount)
	log.Infof("monitoring for synchronization, awaiting %d anchor point messages to become solid", wantedAnchorPointsCount)

	synced := make(chan types.Empty)

	initAnchorPointClosure := events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		defer cachedMessage.Release()
		defer cachedMessageMetadata.Release()
		if addedAnchorID := initAnchorPoint(anchorPoints, cachedMessage.Unwrap()); addedAnchorID != nil {
			anchorPoints.Lock()
			defer anchorPoints.Unlock()
			log.Infof("added message %s as anchor point (%d of %d collected)", addedAnchorID.String()[:10], anchorPoints.collectedCount(), anchorPoints.wanted)
		}
	})

	checkAnchorPointSolidityClosure := events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		defer cachedMessage.Release()
		defer cachedMessageMetadata.Release()
		allSolid, newSolidAnchorID := checkAnchorPointSolidity(anchorPoints, cachedMessage.Unwrap())

		if newSolidAnchorID != nil {
			log.Infof("anchor message %s has become solid", newSolidAnchorID.String()[:10])
		}

		if !allSolid {
			return
		}
		synced <- types.Empty{}
	})

	daemon.BackgroundWorker("Sync-Monitor", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle.Events.MessageAttached.Attach(initAnchorPointClosure)
		defer messagelayer.Tangle.Events.MessageAttached.Detach(initAnchorPointClosure)
		messagelayer.Tangle.Events.MessageSolid.Attach(checkAnchorPointSolidityClosure)
		defer messagelayer.Tangle.Events.MessageSolid.Detach(checkAnchorPointSolidityClosure)

		cleanupDelta := config.Node.GetDuration(CfgSyncAnchorPointsCleanupAfterSec) * time.Second
		ticker := time.NewTimer(config.Node.GetDuration(CfgSyncAnchorPointsCleanupIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				anchorPoints.Lock()
				for id, itGotAdded := range anchorPoints.ids {
					if time.Since(itGotAdded) > cleanupDelta {
						log.Infof("freeing anchor point slot of %s as it didn't become solid within %v", id.String()[:10], cleanupDelta)
						delete(anchorPoints.ids, id)
					}
				}
				anchorPoints.Unlock()
			case <-shutdownSignal:
				return
			case <-synced:
				log.Infof("all anchor messages have become solid, marking node as synced")
				markSynced()
				return
			}
		}
	}, shutdown.PrioritySynchronization)
}

// fills up the anchor points with newly attached messages which then are used to determine whether we are synchronized.
func initAnchorPoint(anchorPoints *anchorpoints, msg *message.Message) *message.Id {
	if synced.Load() {
		return nil
	}

	anchorPoints.Lock()
	defer anchorPoints.Unlock()

	// we don't need to add additional anchor points if the set was already filled once
	if anchorPoints.wasFilled() {
		return nil
	}

	// as a rule, we don't consider messages attaching directly to genesis anchors
	if msg.TrunkId() == message.EmptyId || msg.BranchId() == message.EmptyId {
		return nil
	}

	// add a new anchor point
	id := msg.Id()
	anchorPoints.add(id)
	return &id
}

// checks whether an anchor point message became solid.
// if all anchor points became solid, it sets the node's state to synchronized.
func checkAnchorPointSolidity(anchorPoints *anchorpoints, msg *message.Message) (bool, *message.Id) {
	anchorPoints.Lock()
	defer anchorPoints.Unlock()

	if synced.Load() || len(anchorPoints.ids) == 0 {
		return false, nil
	}

	// check whether an anchor message become solid
	msgID := msg.Id()
	if !anchorPoints.has(msgID) {
		return false, nil
	}

	// an anchor became solid
	anchorPoints.markAsSolidified(msgID)

	if !anchorPoints.wereAllSolidified() {
		return false, &msgID
	}

	// all anchor points have become solid
	return true, &msgID
}

func newAnchorPoints(wantedAnchorPointsCount int) *anchorpoints {
	return &anchorpoints{
		ids:    make(map[message.Id]time.Time),
		wanted: wantedAnchorPointsCount,
	}
}

// anchorpoints are a set of messages which we use to determine whether the node has become synchronized.
type anchorpoints struct {
	sync.Mutex
	// the ids of the anchor points with their addition time.
	ids map[message.Id]time.Time
	// the wanted amount of anchor points which should become solid.
	wanted int
	// how many anchor points have been solidified.
	solidified int
}

// adds the given message to the anchor points set.
func (ap *anchorpoints) add(id message.Id) {
	ap.ids[id] = time.Now()
}

func (ap *anchorpoints) has(id message.Id) bool {
	_, has := ap.ids[id]
	return has
}

// marks the given anchor point as solidified which removes it from the set and bumps the solidified count.
func (ap *anchorpoints) markAsSolidified(id message.Id) {
	delete(ap.ids, id)
	ap.solidified++
}

// tells whether the anchor points set was filled at some point.
func (ap *anchorpoints) wasFilled() bool {
	return ap.collectedCount() == ap.wanted
}

// tells whether all anchor points have become solid.
func (ap *anchorpoints) wereAllSolidified() bool {
	return ap.solidified == ap.wanted
}

// tells the number of effectively collected anchor points.
func (ap *anchorpoints) collectedCount() int {
	// since an anchor point potentially was solidified before the set became full,
	// we need to incorporate that count too
	return ap.solidified + len(ap.ids)
}
