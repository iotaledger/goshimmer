package manualpeering

import (
	"encoding/json"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"golang.org/x/xerrors"
	"sync"

	"github.com/iotaledger/goshimmer/packages/manualpeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Manualpeering"

var (
	// plugin is the plugin instance of the gossip plugin.
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

func log() *logger.Logger {
	return logger.NewLogger(PluginName)
}

func Manager() *manualpeering.Manager {
	managerOnce.Do(createManager)
	return manager
}

func createManager() {
	lPeer := local.GetInstance()
	manager = manualpeering.NewManager(gossip.Manager(), lPeer, log())
}

func configurePlugin(*node.Plugin) {
	configureWebAPI()
}

func runPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, startManager, shutdown.PriorityManualpeering); err != nil {
		log().Panicf("Failed to start as daemon: %s", err)
	}
}

func startManager(shutdownSignal <-chan struct{}) {
	mgr := Manager()
	mgr.Start()
	defer func() {
		if err := mgr.Stop(); err != nil {
			log().Errorw("Failed to stop the manager", "err", err)
		}
	}()
	addPeersFromConfigToManager(mgr)
	<-shutdownSignal
}
func addPeersFromConfigToManager(mgr *manualpeering.Manager) {
	peers, err := getNeighborsFromConfig()
	if err != nil {
		log().Errorw("Failed to get known peers from the config file, continuing without them...", "err", err)
	} else if len(peers) != 0 {
		log().Infow("Pass manual neighbors list to gossip layer", "neighbors", peers)
		mgr.AddPeers(peers)
	}
}

func getNeighborsFromConfig() ([]*peer.Peer, error) {
	rawMap := config.Node().Get(CfgManualpeeringKnownPeers)
	// This is a hack to transform a map from config into peer.Peer struct.
	jsonData, err := json.Marshal(rawMap)
	if err != nil {
		return nil, xerrors.Errorf("can't marshal neighbors map from config into json data: %w", err)
	}
	var peers []*peer.Peer
	if err := json.Unmarshal(jsonData, &peers); err != nil {
		return nil, xerrors.Errorf("can't parse neighbors from json: %w", err)
	}
	return peers, nil
}
