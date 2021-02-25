package syncbeaconfollower

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	syncbeacon_payload "github.com/iotaledger/goshimmer/plugins/syncbeacon/payload"
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

	// CfgSyncBeaconSyncPercentage defines the percentage of following nodes that have to be synced.
	CfgSyncBeaconSyncPercentage = "syncbeaconfollower.syncPercentage"
)

// Status represents the status of a beacon node consisting of latest messageID, sentTime and sync status.
type Status struct {
	MsgID    tangle.MessageID
	SentTime int64
	Synced   bool
}

func init() {
	flag.StringSlice(CfgSyncBeaconFollowNodes, []string{"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24", "9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd"}, "list of trusted nodes to follow their sync status")

	flag.Int(CfgSyncBeaconMaxTimeWindowSec, 10, "the maximum time window for which a sync payload would be considerable")
	flag.Int(CfgSyncBeaconMaxTimeOfflineSec, 70, "the maximum time the node should stay synced without receiving updates")
	flag.Int(CfgSyncBeaconCleanupInterval, 10, "the interval at which cleanups are done")
	flag.Float64(CfgSyncBeaconSyncPercentage, 0.5, "percentage of nodes being followed that need to be synced in order to consider the node synced")
}

var (
	// plugin is the plugin instance of the sync beacon plugin.
	plugin                  *node.Plugin
	once                    sync.Once
	log                     *logger.Logger
	currentBeacons          map[ed25519.PublicKey]*Status
	currentBeaconPubKeys    map[ed25519.PublicKey]string
	mutex                   sync.RWMutex
	beaconMaxTimeOfflineSec float64
	beaconMaxTimeWindowSec  float64
	syncPercentage          float64
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
// it has the relevant past and present message data. The synchronized state is
// defined by following a certain set of sync beacon nodes where for each of these
// beacons a message needs to become solid within bounded time.
func Synced() bool {
	return synced.Load()
}

// SyncStatus returns the detailed status per beacon node.
func SyncStatus() (bool, map[ed25519.PublicKey]Status) {
	mutex.RLock()
	defer mutex.RUnlock()

	beacons := make(map[ed25519.PublicKey]Status)
	for publicKey, status := range currentBeacons {
		beacons[publicKey] = Status{
			MsgID:    status.MsgID,
			SentTime: status.SentTime,
			Synced:   status.Synced,
		}
	}

	return Synced(), beacons
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

// configure plugin
func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	pubKeys := config.Node().Strings(CfgSyncBeaconFollowNodes)
	beaconMaxTimeOfflineSec = float64(config.Node().Int(CfgSyncBeaconMaxTimeOfflineSec))
	beaconMaxTimeWindowSec = float64(config.Node().Int(CfgSyncBeaconMaxTimeWindowSec))
	syncPercentage = config.Node().Float64(CfgSyncBeaconSyncPercentage)
	if syncPercentage < 0.5 || syncPercentage > 1.0 {
		log.Warnf("invalid syncPercentage: %f, syncPercentage has to be in [0.5, 1.0] interval", syncPercentage)
		// set it to default
		log.Warnf("setting syncPercentage to default value of 0.5")
		syncPercentage = 0.5
	}

	currentBeacons = make(map[ed25519.PublicKey]*Status)
	currentBeaconPubKeys = make(map[ed25519.PublicKey]string)

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
		currentBeacons[pubKey] = &Status{
			MsgID:    tangle.EmptyMessageID,
			Synced:   false,
			SentTime: 0,
		}
		currentBeaconPubKeys[pubKey] = str
	}
	if len(currentBeaconPubKeys) == 0 {
		log.Panicf("Follow node list cannot be empty: %w", ErrMissingFollowNodes)
	}

	messagelayer.Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			messagePayload := msg.Payload()
			if messagePayload.Type() != syncbeacon_payload.Type {
				return
			}

			payload, ok := messagePayload.(*syncbeacon_payload.Payload)
			if !ok {
				log.Debug("could not cast payload to sync beacon object")
				return
			}

			// check if issuer is in configured beacon follow list
			if _, ok := currentBeaconPubKeys[msg.IssuerPublicKey()]; !ok {
				return
			}

			// only consider fresh beacons
			mutex.RLock()
			if payload.SentTime() < currentBeacons[msg.IssuerPublicKey()].SentTime {
				mutex.RUnlock()
				return
			}
			mutex.RUnlock()

			handlePayload(payload, msg.IssuerPublicKey(), msg.ID())
		})
	}))
}

// handlePayload handles the received payload. It does the following checks:
// The time that payload was sent is not greater than CfgSyncBeaconMaxTimeWindowSec. If the duration is longer than CfgSyncBeaconMaxTimeWindowSec, we consider that beacon to be out of sync till we receive a newer payload.
// More than syncPercentage of followed nodes are also synced, the node is set to synced. Otherwise, its set as desynced.
func handlePayload(syncBeaconPayload *syncbeacon_payload.Payload, issuerPublicKey ed25519.PublicKey, msgID tangle.MessageID) {
	synced := true
	dur := time.Since(time.Unix(0, syncBeaconPayload.SentTime()))
	if dur.Seconds() > beaconMaxTimeWindowSec {
		log.Debugf("sync beacon %s, received from %s is too old.", msgID, issuerPublicKey)
		synced = false
	}

	mutex.Lock()
	defer mutex.Unlock()
	currentBeacons[issuerPublicKey].Synced = synced
	currentBeacons[issuerPublicKey].MsgID = msgID
	currentBeacons[issuerPublicKey].SentTime = syncBeaconPayload.SentTime()

	updateSynced()
}

// updateSynced checks the beacon nodes and update the nodes sync status
func updateSynced() {
	beaconNodesSyncedCount := 0.0
	for _, status := range currentBeacons {
		if status.Synced {
			beaconNodesSyncedCount++
		}
	}

	var globalSynced bool
	if len(currentBeacons) > 0 {
		globalSynced = beaconNodesSyncedCount/float64(len(currentBeacons)) >= syncPercentage
	}

	OverwriteSyncedState(globalSynced)
}

// cleanupFollowNodes cleans up offline nodes by setting their sync status to false after a configurable time window.
func cleanupFollowNodes() {
	mutex.Lock()
	defer mutex.Unlock()
	for publicKey, status := range currentBeacons {
		if status.MsgID != tangle.EmptyMessageID {
			dur := time.Since(time.Unix(0, status.SentTime))
			if dur.Seconds() > beaconMaxTimeOfflineSec {
				currentBeacons[publicKey].Synced = false
			}
		}
	}
	updateSynced()
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker("Sync-Beacon-Cleanup", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(config.Node().Duration(CfgSyncBeaconCleanupInterval) * time.Second)
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
