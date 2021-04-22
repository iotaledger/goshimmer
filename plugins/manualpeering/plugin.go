package manualpeering

import (
	"encoding/json"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/plugins/config"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/manualpeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/gossip"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the manualpeering plugin.
const PluginName = "Manualpeering"

var (
	// plugin is the plugin instance of the manualpeering plugin.
	plugin      *node.Plugin
	pluginOnce  sync.Once
	manager     *manualpeering.Manager
	managerOnce sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configurePlugin, runPlugin)
	})
	return plugin
}

// Manager is a singleton for manualpeering Manager.
func Manager() *manualpeering.Manager {
	managerOnce.Do(func() {
		lPeer := local.GetInstance()
		manager = manualpeering.NewManager(gossip.Manager(), lPeer, logger.NewLogger(PluginName))
	})
	return manager
}

func configurePlugin(*node.Plugin) {
	configureWebAPI()
}

func runPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, startManager, shutdown.PriorityManualpeering); err != nil {
		plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func startManager(shutdownSignal <-chan struct{}) {
	mgr := Manager()
	mgr.Start()
	defer func() {
		if err := mgr.Stop(); err != nil {
			plugin.Logger().Errorw("Failed to stop the manager", "err", err)
		}
	}()
	addPeersFromConfigToManager(mgr)
	<-shutdownSignal
}

func addPeersFromConfigToManager(mgr *manualpeering.Manager) {
	peers, err := getKnownPeersFromConfig()
	if err != nil {
		plugin.Logger().Errorw("Failed to get known peers from the config file, continuing without them...", "err", err)
	} else if len(peers) != 0 {
		plugin.Logger().Infow("Pass known peers list from the config file to the manager", "peers", peers)
		mgr.AddPeer(peers...)
	}
}

func getKnownPeersFromConfig() ([]*peer.Peer, error) {
	rawMap := config.Node().Get(CfgManualpeeringKnownPeers)
	// This is a hack to transform a map from config into peer.Peer struct.
	jsonData, err := json.Marshal(rawMap)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal known peers map from config into json data")
	}
	var peers []*peer.Peer
	if err := json.Unmarshal(jsonData, &peers); err != nil {
		return nil, errors.Wrap(err, "can't parse peers from json")
	}
	return peers, nil
}
