package sync

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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
	// CfgBootstrapSyncAnchorPointsCount defines the amount of anchor points to use to determine
	// whether a node is synchronized.
	CfgSyncAnchorPointsCount = "sync.anchorPointsCount"
)

func init() {
	flag.Int(CfgSyncAnchorPointsCount, 3, "the amount of anchor points to use to determine whether a node is synchronized")
}

var (
	// Plugin is the plugin instance of the sync plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure)
	// ErrNodeNotSynchronized is returned when an operation can't be executed because
	// the node is not synchronized.
	ErrNodeNotSynchronized = errors.New("node is not synchronized")
	log                    *logger.Logger
	synchronized           atomic.Bool
	anchorPoints           *anchorpoints
	detachHandlers         func()
)

// Synced tells whether the node is in a state we consider synchronized, meaning
// it has the relevant past and present message data.
func Synced() bool {
	return synchronized.Load()
}

// OverwriteSyncedState overwrites the synced state with the given value.
func OverwriteSyncedState(synced bool) {
	synchronized.Store(synced)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	// another plugin might overwrite the sync state while configuring itself,
	// for example the bootstrap plugin, implying that we don't need to register any handlers
	if Synced() {
		return
	}

	wantedAnchorPointsCount := config.Node.GetInt(CfgSyncAnchorPointsCount)
	anchorPoints = &anchorpoints{
		ids:    make(map[message.Id]types.Empty),
		wanted: wantedAnchorPointsCount,
	}

	log.Infof("starting node in non-bootstrapping mode, awaiting %d anchor point messages to become solid", wantedAnchorPointsCount)

	// only register anchor event handlers when we're not in bootstrap mode
	detachHandlers = registerMessageHandlers()
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
