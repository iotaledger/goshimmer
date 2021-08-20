package manualpeering

import (
	"encoding/json"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/manualpeering"
	"github.com/iotaledger/goshimmer/packages/shutdown"
)

// PluginName is the name of the manualpeering plugin.
const PluginName = "Manualpeering"

var (
	// Plugin is the plugin instance of the manualpeering plugin.
	Plugin      *node.Plugin
	deps        = new(dependencies)
	manager     *manualpeering.Manager
	managerOnce sync.Once
)

type dependencies struct {
	dig.In

	Local     *peer.Local
	GossipMgr *gossip.Manager
	Server    *echo.Echo
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

// Manager is a singleton for manualpeering Manager.
func Manager() *manualpeering.Manager {
	managerOnce.Do(func() {
		lPeer := deps.Local
		manager = manualpeering.NewManager(deps.GossipMgr, lPeer, logger.NewLogger(PluginName))
	})
	return manager
}

func configure(_ *node.Plugin) {
	configureWebAPI()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, startManager, shutdown.PriorityManualpeering); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func startManager(shutdownSignal <-chan struct{}) {
	mgr := Manager()
	mgr.Start()
	defer func() {
		if err := mgr.Stop(); err != nil {
			Plugin.Logger().Errorw("Failed to stop the manager", "err", err)
		}
	}()
	addPeersFromConfigToManager(mgr)
	<-shutdownSignal
}

func addPeersFromConfigToManager(mgr *manualpeering.Manager) {
	peers, err := getKnownPeersFromConfig()
	if err != nil {
		Plugin.Logger().Errorw("Failed to get known peers from the config file, continuing without them...", "err", err)
	} else if len(peers) != 0 {
		Plugin.Logger().Infow("Pass known peers list from the config file to the manager", "peers", peers)
		if err := mgr.AddPeer(peers...); err != nil {
			Plugin.Logger().Infow("Failed to pass known peers list from the config file to the manager",
				"peers", peers, "err", err)
		}
	}
}

func getKnownPeersFromConfig() ([]*manualpeering.KnownPeerToAdd, error) {
	if Parameters.KnownPeers == "" {
		return []*manualpeering.KnownPeerToAdd{}, nil
	}
	var peers []*manualpeering.KnownPeerToAdd
	if err := json.Unmarshal([]byte(Parameters.KnownPeers), &peers); err != nil {
		return nil, errors.Wrap(err, "can't parse peers from json")
	}
	return peers, nil
}
