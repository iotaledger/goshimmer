package bootstrap

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/spammer"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/types"
	flag "github.com/spf13/pflag"
	"go.uber.org/atomic"
)

const (
	// PluginName is the plugin name of the bootstrap plugin.
	PluginName = "Bootstrap"
	// CfgBootstrapMode defines whether the node starts in bootstrapping mode.
	CfgBootstrapMode = "bootstrap.mode"
	// CfgBootstrapInitialIssuanceTimePeriodSec defines the initial time period of how long the node should be
	// issuing messages when started in bootstrapping mode. If the value is set to -1, the issuance is continuous.
	CfgBootstrapInitialIssuanceTimePeriodSec = "bootstrap.initialIssuance.timePeriodSec"
	// CfgBootstrapSyncAnchorPointsCount defines the amount of anchor points to use to determine whether a node is
	// synchronized when not running in bootstrapping mode.
	CfgBootstrapSyncAnchorPointsCount = "bootstrap.syncAnchorPointsCount"
	// the messages per second to issue when in bootstrapping mode.
	initialIssuanceMPS = 1
	// the value which determines a continuous issuance of messages from the bootstrap plugin.
	continuousIssuance = -1
)

func init() {
	flag.Bool(CfgBootstrapMode, false, "whether the node should be started in bootstrapping mode or not.")
	flag.Int(CfgBootstrapInitialIssuanceTimePeriodSec, -1, "the initial time period of how long the node should be issuing messages when started in bootstrapping mode. "+
		"If the value is set to -1, the issuance is continuous.")
	flag.Int(CfgBootstrapSyncAnchorPointsCount, 3, "the amount of anchor points to use to determine whether a node is synchronized when not running in bootstrapping mode")
}

var (
	// Plugin is the plugin instance of the bootstrap plugin.
	Plugin         = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log            *logger.Logger
	synchronized   atomic.Bool
	anchorPoints   *anchorpoints
	detachHandlers func()
)

// Synchronized tells whether the node is in a state we consider synchronized, meaning
// it has the relevant past and present message data.
func Synchronized() bool {
	return synchronized.Load()
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	if config.Node.GetBool(CfgBootstrapMode) {
		log.Infof("starting node in bootstrapping mode")
		// auto. synced if in bootstrapping mode
		synchronized.Store(true)
		return
	}

	wantedAnchorPointsCount := config.Node.GetInt(CfgBootstrapSyncAnchorPointsCount)
	anchorPoints = &anchorpoints{
		ids:    make(map[message.Id]types.Empty),
		wanted: wantedAnchorPointsCount,
	}

	log.Infof("starting node in non-bootstrapping mode, awaiting %d anchor point messages to become solid", wantedAnchorPointsCount)

	// only register anchor event handlers when we're not in bootstrap mode
	detachHandlers = registerMessageHandlers()
}

func run(_ *node.Plugin) {
	if !config.Node.GetBool(CfgBootstrapMode) {
		return
	}

	messageSpammer := spammer.New(messagelayer.MessageFactory)
	issuancePeriodSec := config.Node.GetInt(CfgBootstrapInitialIssuanceTimePeriodSec)
	issuancePeriod := time.Duration(issuancePeriodSec) * time.Second

	// issue messages on top of the genesis
	_ = daemon.BackgroundWorker("Bootstrapping-Issuer", func(shutdownSignal <-chan struct{}) {
		messageSpammer.Start(initialIssuanceMPS)
		defer messageSpammer.Shutdown()
		// don't stop issuing messages if in continuous mode
		if issuancePeriodSec == continuousIssuance {
			log.Info("continuously issuing bootstrapping messages")
			<-shutdownSignal
			return
		}
		log.Infof("issuing bootstrapping messages for %d seconds", issuancePeriodSec)
		select {
		case <-time.After(issuancePeriod):
		case <-shutdownSignal:
		}
	})
}

// registers the event handler for checking the anchor message state
// and returns a function to detach those event handlers.
func registerMessageHandlers() func() {
	initAnchorPointClosure := events.NewClosure(initAnchorPoint)
	checkAnchorPointSolidityClosure := events.NewClosure(checkAnchorPointSolidity)
	messagelayer.Tangle.Events.MessageAttached.Attach(initAnchorPointClosure)
	messagelayer.Tangle.Events.MessageSolid.Attach(checkAnchorPointSolidityClosure)
	return func() {
		messagelayer.Tangle.Events.MessageAttached.Detach(initAnchorPointClosure)
		messagelayer.Tangle.Events.MessageSolid.Detach(checkAnchorPointSolidityClosure)
	}
}

// fills up the anchor points with newly attached messages which then are used to determine whether we are synchronized.
func initAnchorPoint(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()
	if synchronized.Load() {
		return
	}

	anchorPoints.Lock()
	defer anchorPoints.Unlock()

	// we don't need to add additional anchor points if the set was already filled once
	if anchorPoints.wasFilled() {
		return
	}

	// as a rule, we don't consider messages attaching directly to genesis anchors
	msg := cachedMessage.Unwrap()
	if msg.TrunkId() == message.EmptyId || msg.BranchId() == message.EmptyId {
		return
	}

	// add a new anchor point
	anchorPoints.add(msg.Id())
	log.Infof("added message %s as synchronization anchor point (%d of %d collected)", msg.Id().String()[:10], anchorPoints.collectedCount(), anchorPoints.wanted)
}

// checks whether an anchor point message became solid.
// if all anchor points became solid, it sets the node's state to synchronized.
func checkAnchorPointSolidity(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	anchorPoints.Lock()
	defer anchorPoints.Unlock()

	if synchronized.Load() || len(anchorPoints.ids) == 0 {
		return
	}

	// check whether an anchor message become solid
	msgID := cachedMessage.Unwrap().Id()
	if !anchorPoints.has(msgID) {
		return
	}

	// an anchor became solid
	log.Infof("anchor message %s has become solid", msgID.String()[:10])
	anchorPoints.markAsSolidified(msgID)

	if !anchorPoints.wereAllSolidified() {
		return
	}

	// all anchor points have become solid
	log.Infof("all anchor messages have become solid, setting node as synchronized")
	synchronized.Store(true)

	// since we now are synchronized, we no longer need to listen to this events,
	// however, we need to detach in a separate goroutine, since this function
	// runs within the event handlers loop
	if detachHandlers == nil {
		return
	}
	go detachHandlers()
}

// anchorpoints are a set of messages which we use to determine whether the node has become synchronized.
type anchorpoints struct {
	sync.Mutex
	// the ids of the anchor points.
	ids map[message.Id]types.Empty
	// the wanted amount of anchor points which should become solid.
	wanted int
	// how many anchor points have been solidified.
	solidified int
}

// adds the given message to the anchor points set.
func (ap *anchorpoints) add(id message.Id) {
	ap.ids[id] = types.Empty{}
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
