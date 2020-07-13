package syncbeacon

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	gossipPkg "github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
	goSync "sync"
	"time"
)

const (
	// PluginName is the plugin name of the sync beacon plugin.
	PluginName = "Sync Beacon"

	// CfgSyncBeaconFollowNodes defines the list of nodes this node should follow to determine its sync status.
	CfgSyncBeaconFollowNodes = "syncbeacon.followNodes"

	// CfgSyncBeaconPrimary defines whether this node should be a beacon.
	// If set to true, the node will broadcast its sync status on every heartbeat to its followers.
	CfgSyncBeaconPrimary = "syncbeacon.primary"

	// CfgSyncBeaconMaxTimeWindowSec defines the maximum time window for which a sync payload would be considerable.
	CfgSyncBeaconMaxTimeWindowSec = "syncbeacon.maxTimeWindowSec"

	// CfgSyncBeaconBroadcastIntervalSec is the interval in seconds at which the node broadcasts its sync status.
	CfgSyncBeaconBroadcastIntervalSec = "syncbeacon.broadcastInterval"

	// The percentage of following nodes that have to be synced
	syncPercentage = 2.0 / 3.0
)

func init() {
	// TODO: Check the defaults here
	flag.StringSlice(CfgSyncBeaconFollowNodes, []string{"2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQ"}, "list of trusted nodes to follow their sync status")
	flag.Bool(CfgSyncBeaconPrimary, false, "if this node should be a sync beacon")
	flag.Int(CfgSyncBeaconBroadcastIntervalSec, 30, "the interval at which the node will broadcast ist sync status")
	flag.Int(CfgSyncBeaconMaxTimeWindowSec, 10, "the maximum time window for which a sync payload would be considerable")
}

var (
	// plugin is the plugin instance of the sync beacon plugin.
	plugin               *node.Plugin
	once                 goSync.Once
	log                  *logger.Logger
	mgr                  *gossipPkg.Manager
	beaconNodesStatusMap map[string]Payload
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
}

// configure events
func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	gossip.Manager().Events().MessageReceived.Attach(events.NewClosure(func(event gossipPkg.MessageReceivedEvent) {
		messagelayer.MessageParser().Parse(event.Data, event.Peer)
	}))
	messagelayer.MessageParser().Events.MessageParsed.Attach(events.NewClosure(func(message *message.Message, peer *peer.Peer) {
		handleMessage(message, peer)
	}))
}

// handleMessage handles the received message. It does the following checks:
// - The payload is a syncbeacon payload. Then it checks that the issuer of the payload is a followed node.
// - The time that payload was sent is not greater than  CfgSyncBeaconMaxTimeWindowSec. If the duration is longer than CfgSyncBeaconMaxTimeWindowSec, we consider that beacon to be out of sync till we receive a newer payload.
// - If the node is synced and more than syncPercentage of followed nodes are also synced, the node is set to synced. Otherwise, its set as desynced.
func handleMessage(message *message.Message, peer *peer.Peer) {
	if !IsSyncBeaconPayload(message) {
		return
	}
	peerPublicKey := peer.PublicKey().String()
	beaconNodesPublicKeys := config.Node().GetStringSlice(CfgSyncBeaconFollowNodes)
	beaconNodesStatusMap = make(map[string]Payload, len(beaconNodesPublicKeys))

	//check if peer is in configured beacon follow list
	shouldAccept := false
	for i := 0; i < len(beaconNodesPublicKeys); i++ {
		if beaconNodesPublicKeys[i] == peerPublicKey {
			shouldAccept = true
			break
		}
	}
	if !shouldAccept {
		return
	}

	syncBeaconPayload := &Payload{}
	err := syncBeaconPayload.Unmarshal(message.Payload().Bytes())
	if err != nil {
		log.Errorf("error unmarshalling syncbeacon payload %s", err)
		return
	}

	log.Infof("syncbeacon payload received from %s", peerPublicKey)

	dur := time.Now().Sub(time.Unix(0, syncBeaconPayload.sentTime))
	if dur.Seconds() > float64(config.Node().GetInt(CfgSyncBeaconMaxTimeWindowSec)) {
		log.Infof("syncbeacon payload received from %s is too old", peerPublicKey)
		syncBeaconPayload.syncStatus = false
	}

	beaconNodesStatusMap[peerPublicKey] = *syncBeaconPayload
	beaconNodesSyncedCount := 0.0
	for _, p := range beaconNodesStatusMap {
		if p.SyncStatus() {
			beaconNodesSyncedCount++
		}
	}
	isLocalSynced := sync.Synced()
	isBeaconSynced := (beaconNodesSyncedCount / float64(len(beaconNodesPublicKeys))) > syncPercentage
	if isLocalSynced && isBeaconSynced {
		sync.MarkSynced()
	} else {
		sync.MarkDesynced()
	}
}

// broadcastSyncBeaconPayload broadcasts its sync status to its neighbors.
func broadcastSyncBeaconPayload() {
	syncBeaconPayload := NewSyncBeaconPayload(sync.Synced(), time.Now().UnixNano())
	msg := message.New(message.EmptyId, message.EmptyId, time.Now(), ed25519.PublicKey{}, 0, syncBeaconPayload, 0, ed25519.Signature{})
	mgr.SendMessage(msg.Bytes())
}

func run(_ *node.Plugin) {
	if !config.Node().GetBool(CfgSyncBeaconPrimary) {
		return
	}
	if err := daemon.BackgroundWorker("Sync-Beacon", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(config.Node().GetDuration(CfgSyncBeaconBroadcastIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				broadcastSyncBeaconPayload()
			case <-shutdownSignal:
				return
			}
		}
	}, shutdown.PrioritySynchronization); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
