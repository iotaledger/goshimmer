package syncbeaconfollower

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/syncbeacon"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
	"go.uber.org/atomic"
)

const (
	// PluginName is the plugin name of the sync beacon plugin.
	PluginName = "SyncBeaconFollower"

	// CfgSyncBeaconFollowNodes defines the list of nodes this node should follow to determine its sync status.
	CfgSyncBeaconFollowNodes = "syncbeaconfollower.followNodes"

	// CfgSyncBeaconMaxTimeWindowSec defines the maximum time window for which a sync payload would be considerable.
	CfgSyncBeaconMaxTimeWindowSec = "syncbeaconfollower.maxTimeWindowSec"

	// CfgSyncBeaconMaxTimeOfflineSec defines the maximum time a beacon node can stay without receiving updates.
	CfgSyncBeaconMaxTimeOfflineSec = "syncbeaconfollower.maxTimeOffline"

	// CfgSyncBeaconCleanupInterval defines the interval that old beacon status are cleaned up.
	CfgSyncBeaconCleanupInterval = "syncbeaconfollower.cleanupInterval"

	// The percentage of following nodes that have to be synced.
	syncPercentage = 0.6
)

// beaconSync represents the beacon payload and the sync status.
type beaconSync struct {
	payload *syncbeacon.Payload
	synced  bool
}

func init() {
	//flag.StringSlice(CfgSyncBeaconFollowNodes, []string{"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24", "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd"}, "list of trusted nodes to follow their sync status")
	flag.StringSlice(CfgSyncBeaconFollowNodes, []string{}, "list of trusted nodes to follow their sync status")

	flag.Int(CfgSyncBeaconMaxTimeWindowSec, 10, "the maximum time window for which a sync payload would be considerable")
	flag.Int(CfgSyncBeaconMaxTimeOfflineSec, 40, "the maximum time the node should stay synced without receiving updates")
	flag.Int(CfgSyncBeaconCleanupInterval, 10, "the interval at which cleanups are done")
}

var (
	// plugin is the plugin instance of the sync beacon plugin.
	plugin                  *node.Plugin
	once                    sync.Once
	log                     *logger.Logger
	currentBeacons          map[ed25519.PublicKey]*beaconSync
	currentBeaconPubKeys    map[ed25519.PublicKey]string
	mutex                   sync.RWMutex
	beaconMaxTimeOfflineSec float64
	// tells whether the node is synced or not.
	synced atomic.Bool

	// ErrMissingFollowNodes is returned if the node starts with no follow nodes list
	ErrMissingFollowNodes = errors.New("follow nodes list is required")
	// ErrNodeNotSynchronized is returned when an operation can't be executed because
	// the node is not synchronized.
	ErrNodeNotSynchronized = errors.New("node is not synchronized")
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Synced tells whether the node is in a state we consider synchronized, meaning
// it has the relevant past and present message data.
func Synced() bool {
	return synced.Load()
}

// MarkSynced marks the node as synced.
func MarkSynced() {
	synced.Store(true)
}

// MarkDesynced marks the node as desynced.
func MarkDesynced() {
	synced.Store(false)
}

// OverwriteSyncedState overwrites the synced state with the given value.
func OverwriteSyncedState(syncedOverwrite bool) {
	synced.Store(syncedOverwrite)
}

// configure events
func configure(_ *node.Plugin) {
	pubKeys := config.Node().GetStringSlice(CfgSyncBeaconFollowNodes)
	beaconMaxTimeOfflineSec = float64(config.Node().GetInt(CfgSyncBeaconMaxTimeOfflineSec))
	log = logger.NewLogger(PluginName)

	currentBeacons = make(map[ed25519.PublicKey]*beaconSync, len(pubKeys))
	currentBeaconPubKeys = make(map[ed25519.PublicKey]string, len(pubKeys))

	for _, str := range pubKeys {
		bytes, err := base58.Decode(str)
		if err != nil {
			log.Warnf("error decoding public key: %w", err)
			continue
		}
		pubKey, _, err := ed25519.PublicKeyFromBytes(bytes)
		if err != nil {
			log.Warnf("%s is not a valid public key: %w", err)
			continue
		}
		currentBeacons[pubKey] = &beaconSync{
			payload: nil,
			synced:  false,
		}
		currentBeaconPubKeys[pubKey] = str
	}
	if len(currentBeaconPubKeys) == 0 {
		log.Panicf("Follow node list cannot be empty: %w", ErrMissingFollowNodes)
	}

	messagelayer.Tangle().Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			messagePayload := msg.Payload()
			if messagePayload.Type() != syncbeacon.Type {
				return
			}

			payload, ok := messagePayload.(*syncbeacon.Payload)
			if !ok {
				log.Info("could not cast payload to sync beacon object")
				return
			}

			//check if issuer is in configured beacon follow list
			if _, ok := currentBeaconPubKeys[msg.IssuerPublicKey()]; !ok {
				return
			}

			handlePayload(payload, msg.IssuerPublicKey(), msg.Id())
		})
	}))
}

// handlePayload handles the received payload. It does the following checks:
// It checks that the issuer of the payload is a followed node.
// The time that payload was sent is not greater than  CfgSyncBeaconMaxTimeWindowSec. If the duration is longer than CfgSyncBeaconMaxTimeWindowSec, we consider that beacon to be out of sync till we receive a newer payload.
// More than syncPercentage of followed nodes are also synced, the node is set to synced. Otherwise, its set as desynced.
func handlePayload(syncBeaconPayload *syncbeacon.Payload, issuerPublicKey ed25519.PublicKey, msgId message.Id) {
	mutex.Lock()
	defer mutex.Unlock()

	synced := true
	dur := time.Since(time.Unix(0, syncBeaconPayload.SentTime()))
	if dur.Seconds() > float64(config.Node().GetInt(CfgSyncBeaconMaxTimeWindowSec)) {
		log.Infof("sync beacon %s, received from %s is too old.", msgId, issuerPublicKey)
		synced = false
	}

	currentBeacons[issuerPublicKey].synced = synced
	currentBeacons[issuerPublicKey].payload = syncBeaconPayload

	// TODO: check performance for this, only update node status periodically?
	updateSynced()
}

// updateSynced checks the beacon nodes and update the nodes sync status
func updateSynced() {
	beaconNodesSyncedCount := 0.0
	for _, beaconSync := range currentBeacons {
		if beaconSync.synced {
			beaconNodesSyncedCount++
		}
	}
	synced := true
	if len(currentBeacons) > 0 {
		synced = beaconNodesSyncedCount/float64(len(currentBeacons)) >= syncPercentage
	}

	if synced {
		MarkSynced()
	} else {
		MarkDesynced()
	}
}

// cleanupFollowNodes cleans up offline nodes by setting their sync status to false after a configurable time window.
func cleanupFollowNodes() {
	mutex.Lock()
	defer mutex.Unlock()
	for publicKey, beaconSync := range currentBeacons {
		if beaconSync.payload != nil {
			dur := time.Since(time.Unix(0, beaconSync.payload.SentTime()))
			if dur.Seconds() > beaconMaxTimeOfflineSec {
				beaconSync.synced = false
				currentBeacons[publicKey] = beaconSync
			}
		}
	}
	updateSynced()
}

func run(_ *node.Plugin) {
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
