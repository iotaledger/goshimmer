package manualpeering

import (
	"context"
	"encoding/json"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/manualpeering"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
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
	Server           *echo.Echo
	ManualPeeringMgr *manualpeering.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		newManager := func(lPeer *peer.Local, p2pMgr *p2p.Manager) *manualpeering.Manager {
			return manualpeering.NewManager(p2pMgr, lPeer, event.Plugin.WorkerPool, logger.NewLogger(PluginName))
		}

		if err := event.Container.Provide(newManager); err != nil {
			Plugin.Panic(err)
		}
	})
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
