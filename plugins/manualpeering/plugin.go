package manualpeering

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/node/gossip"
	"github.com/iotaledger/goshimmer/packages/node/manualpeering"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

// PluginName is the name of the manual peering plugin.
const PluginName = "ManualPeering"

var (
	// Plugin is the plugin instance of the manual peering plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Local            *peer.Local
	GossipMgr        *gossip.Manager
	Server           *echo.Echo
	ManualPeeringMgr *manualpeering.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newManager); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func newManager(lPeer *peer.Local, p2pMgr *p2p.Manager) *manualpeering.Manager {
	return manualpeering.NewManager(p2pMgr, lPeer, logger.NewLogger(PluginName))
}

func configure(_ *node.Plugin) {
	configureWebAPI()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, startManager, shutdown.PriorityManualpeering); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func startManager(ctx context.Context) {
	mgr := deps.ManualPeeringMgr
	mgr.Start()
	defer func() {
		if err := mgr.Stop(); err != nil {
			Plugin.Logger().Errorw("Failed to stop the manager", "err", err)
		}
	}()
	addPeersFromConfigToManager(mgr)
	<-ctx.Done()
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
