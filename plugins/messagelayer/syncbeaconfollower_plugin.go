package messagelayer

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	syncbeacon_payload "github.com/iotaledger/goshimmer/plugins/syncbeacon/payload"
)

const (
	// SyncBeaconFollowerPluginName is the plugin name of the sync beacon follower plugin.
	SyncBeaconFollowerPluginName = "SyncBeaconFollower"
)

// Status represents the status of a beacon node consisting of latest messageID, sentTime and sync status.
type Status struct {
	MsgID    tangle.MessageID
	SentTime int64
	Synced   bool
}

var (
	// plugin is the plugin instance of the sync beacon plugin.
	syncBeaconFollowerPlugin     *node.Plugin
	syncBeaconFollowerPluginOnce sync.Once
	currentBeacons               map[ed25519.PublicKey]*Status
	currentBeaconPubKeys         map[ed25519.PublicKey]string
	mutex                        sync.RWMutex

	// ErrMissingFollowNodes is returned if the node starts with no follow nodes list
	ErrMissingFollowNodes = errors.New("follow nodes list is required")
)

// SyncBeaconFollowerPlugin gets the plugin instance.
func SyncBeaconFollowerPlugin() *node.Plugin {
	syncBeaconFollowerPluginOnce.Do(func() {
		syncBeaconFollowerPlugin = node.NewPlugin(SyncBeaconFollowerPluginName, node.Enabled, configureSyncBeaconFollower, runSyncBeaconFollower)
	})
	return syncBeaconFollowerPlugin
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

	return Tangle().Synced(), beacons
}

// configure plugin
func configureSyncBeaconFollower(*node.Plugin) {
	if SyncBeaconFollowerParameters.SyncPercentage < 0.5 || SyncBeaconFollowerParameters.SyncPercentage > 1.0 {
		syncBeaconFollowerPlugin.LogWarnf("invalid syncPercentage: %f, syncPercentage has to be in [0.5, 1.0] interval", SyncBeaconFollowerParameters.SyncPercentage)
		// set it to default
		syncBeaconFollowerPlugin.LogWarnf("setting syncPercentage to default value of 0.5")
		SyncBeaconFollowerParameters.SyncPercentage = 0.5
	}

	currentBeacons = make(map[ed25519.PublicKey]*Status)
	currentBeaconPubKeys = make(map[ed25519.PublicKey]string)

	for _, str := range SyncBeaconFollowerParameters.FollowNodes {
		bytes, err := base58.Decode(str)
		if err != nil {
			syncBeaconFollowerPlugin.LogWarnf("error decoding public key: %w", err)
			continue
		}
		pubKey, _, err := ed25519.PublicKeyFromBytes(bytes)
		if err != nil {
			syncBeaconFollowerPlugin.LogWarnf("%s is not a valid public key: %w", err)
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
		syncBeaconFollowerPlugin.Panicf("Follow node list cannot be empty: %w", ErrMissingFollowNodes)
	}

	Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
			messagePayload := msg.Payload()
			if messagePayload.Type() != syncbeacon_payload.Type {
				return
			}

			payload, ok := messagePayload.(*syncbeacon_payload.Payload)
			if !ok {
				syncBeaconFollowerPlugin.LogDebug("could not cast payload to sync beacon object")
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
	dur := clock.Since(time.Unix(0, syncBeaconPayload.SentTime()))
	if dur.Seconds() > float64(SyncBeaconFollowerParameters.MaxTimeWindowSec) {
		syncBeaconFollowerPlugin.LogDebugf("sync beacon %s, received from %s is too old.", msgID, issuerPublicKey)
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
		globalSynced = beaconNodesSyncedCount/float64(len(currentBeacons)) >= SyncBeaconFollowerParameters.SyncPercentage
	}

	Tangle().SetSynced(globalSynced)
}

// cleanupFollowNodes cleans up offline nodes by setting their sync status to false after a configurable time window.
func cleanupFollowNodes() {
	mutex.Lock()
	defer mutex.Unlock()
	for publicKey, status := range currentBeacons {
		if status.MsgID != tangle.EmptyMessageID {
			dur := clock.Since(time.Unix(0, status.SentTime))
			if dur.Seconds() > float64(SyncBeaconFollowerParameters.MaxTimeOffline) {
				currentBeacons[publicKey].Synced = false
			}
		}
	}
	updateSynced()
}

func runSyncBeaconFollower(*node.Plugin) {
	if err := daemon.BackgroundWorker("Sync-Beacon-Cleanup", func(shutdownSignal <-chan struct{}) {
		ticker := time.NewTicker(time.Duration(SyncBeaconFollowerParameters.CleanupInterval) * time.Second)
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
		syncBeaconFollowerPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}
