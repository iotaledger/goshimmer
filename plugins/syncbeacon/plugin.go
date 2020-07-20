package syncbeacon

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/sync"
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

	// CfgSyncBeaconMaxTimeOfflineSec defines the maximum time a beacon node can stay without receiving updates.
	CfgSyncBeaconMaxTimeOfflineSec = "syncbeacon.maxTimeOffline"

	// CfgSyncBeaconCleanupInterval defines the interval that old beacon status are cleaned up.
	CfgSyncBeaconCleanupInterval = "syncbeacon.cleanupInterval"

	// The percentage of following nodes that have to be synced.
	syncPercentage = 0.6
)

// beaconSync represents the beacon payload and the sync status.
type beaconSync struct {
	payload *Payload
	synced  bool
}

func init() {
	flag.StringSlice(CfgSyncBeaconFollowNodes, []string{"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24", "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd"}, "list of trusted nodes to follow their sync status")
	flag.Bool(CfgSyncBeaconPrimary, false, "if this node should be a sync beacon")
	flag.Int(CfgSyncBeaconBroadcastIntervalSec, 30, "the interval at which the node will broadcast ist sync status")
	flag.Int(CfgSyncBeaconMaxTimeWindowSec, 10, "the maximum time window for which a sync payload would be considerable")
	flag.Int(CfgSyncBeaconMaxTimeOfflineSec, 40, "the maximum time the node should stay without receiving updates")
	flag.Int(CfgSyncBeaconCleanupInterval, 10, "the interval at which cleanups are done")
}

var (
	// plugin is the plugin instance of the sync beacon plugin.
	plugin                *node.Plugin
	once                  goSync.Once
	log                   *logger.Logger
	beaconSyncMap         map[string]beaconSync
	beaconNodesPublicKeys []string
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
	beaconNodesPublicKeys = config.Node().GetStringSlice(CfgSyncBeaconFollowNodes)
	beaconSyncMap = make(map[string]beaconSync, len(beaconNodesPublicKeys))

	messagelayer.Tangle().Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			payload := &Payload{}
			err := payload.Unmarshal(msg.Payload().Bytes())
			if err != nil {
				return
			}
			if !IsSyncBeaconPayload(payload) {
				return
			}

			handlePayload(payload, msg.IssuerPublicKey())
		})
	}))
}

// handlePayload handles the received payload. It does the following checks:
// - It checks that the issuer of the payload is a followed node.
// - The time that payload was sent is not greater than  CfgSyncBeaconMaxTimeWindowSec. If the duration is longer than CfgSyncBeaconMaxTimeWindowSec, we consider that beacon to be out of sync till we receive a newer payload.
// - More than syncPercentage of followed nodes are also synced, the node is set to synced. Otherwise, its set as desynced.
func handlePayload(syncBeaconPayload *Payload, issuerPublicKey ed25519.PublicKey) {
	issuerPublicKeyStr := issuerPublicKey.String()
	//check if issuer is in configured beacon follow list
	shouldAccept := false
	for i := 0; i < len(beaconNodesPublicKeys); i++ {
		if beaconNodesPublicKeys[i] == issuerPublicKeyStr {
			shouldAccept = true
			break
		}
	}
	if !shouldAccept {
		return
	}
	_beaconSync := beaconSync{
		payload: syncBeaconPayload,
		synced:  true,
	}

	dur := time.Since(time.Unix(0, syncBeaconPayload.sentTime))
	if dur.Seconds() > float64(config.Node().GetInt(CfgSyncBeaconMaxTimeWindowSec)) {
		log.Infof("syncbeacon received from %s is too old", issuerPublicKey)
		_beaconSync.synced = false
	}

	beaconSyncMap[issuerPublicKeyStr] = _beaconSync
	updateSynced()
}

// updateSynced checks the beacon nodes and update the nodes sync status
func updateSynced() {
	beaconNodesSyncedCount := 0.0
	for _, beaconSync := range beaconSyncMap {
		if beaconSync.synced {
			beaconNodesSyncedCount++
		}
	}
	synced := true
	if len(beaconSyncMap) > 0 {
		synced = beaconNodesSyncedCount/float64(len(beaconSyncMap)) >= syncPercentage
	}

	isLocalSynced := sync.Synced()
	if isLocalSynced && synced {
		sync.MarkSynced()
	} else {
		sync.MarkDesynced()
	}
}

// broadcastSyncBeaconPayload broadcasts its sync status to its neighbors.
func broadcastSyncBeaconPayload() {
	syncBeaconPayload := NewSyncBeaconPayload(time.Now().UnixNano())
	_, err := issuer.IssuePayload(syncBeaconPayload)
	if err != nil {
		log.Info("sync beacon issued")
	}
}

// cleanupFollowNodes cleans up offline nodes by setting their sync status to false after a configurable time window.
func cleanupFollowNodes() {
	for publicKey, beaconSync := range beaconSyncMap {
		dur := time.Since(time.Unix(0, beaconSync.payload.sentTime))
		if dur.Seconds() > float64(config.Node().GetInt(CfgSyncBeaconMaxTimeOfflineSec)) {
			beaconSync.synced = false
			beaconSyncMap[publicKey] = beaconSync
		}
	}
	updateSynced()
}

func run(_ *node.Plugin) {
	if config.Node().GetBool(CfgSyncBeaconPrimary) {
		if err := daemon.BackgroundWorker("Sync-Beacon-Broadcast", func(shutdownSignal <-chan struct{}) {
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

	if err := daemon.BackgroundWorker("Sync-Beacon-Cleanup", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(config.Node().GetDuration(CfgSyncBeaconCleanupInterval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cleanupFollowNodes()
			case <-shutdownSignal:
				return
			}
		}
	}, shutdown.PrioritySynchronization); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
