package firewall

import (
	"context"

	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/app/firewall"
	"github.com/iotaledger/goshimmer/packages/app/ratelimiter"
	"github.com/iotaledger/goshimmer/packages/node/gossip"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Firewall"

var (
	// Plugin is the plugin instance of the gossip plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	GossipMgr *gossip.Manager
	Server    *echo.Echo
	Firewall  *firewall.Firewall
	Tangle    *tangleold.Tangle
}

type firewallDeps struct {
	dig.In
	AutopeeringMgr *selection.Protocol `optional:"true"`
	P2PMgr         *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createFirewall); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func createFirewall(fDeps firewallDeps) *firewall.Firewall {
	f, err := firewall.NewFirewall(fDeps.P2PMgr, fDeps.AutopeeringMgr, Plugin.Logger())
	if err != nil {
		Plugin.LogFatalfAndExit("Couldn't initialize firewall instance: %+v", err)
	}
	return f
}

func configure(_ *node.Plugin) {
	configureWebAPI()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityFirewall); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func start(ctx context.Context) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")

	if mrl := deps.GossipMgr.BlocksRateLimiter(); mrl != nil {
		mrlClosure := event.NewClosure(func(event *ratelimiter.HitEvent) {
			if !deps.Tangle.Bootstrapped() {
				return
			}
			deps.Firewall.HandleFaultyPeer(event.Peer.ID(), &firewall.FaultinessDetails{
				Reason: "Blocks rate limit hit",
				Info: map[string]interface{}{
					"rateLimit": event.RateLimit,
				},
			})
		})
		mrl.Events.Hit.Attach(mrlClosure)
		defer mrl.Events.Hit.Detach(mrlClosure)
	}
	if mrrl := deps.GossipMgr.BlockRequestsRateLimiter(); mrrl != nil {
		mrlClosure := event.NewClosure(func(event *ratelimiter.HitEvent) {
			deps.Firewall.HandleFaultyPeer(event.Peer.ID(), &firewall.FaultinessDetails{
				Reason: "Block requests rate limit hit",
				Info: map[string]interface{}{
					"rateLimit": event.RateLimit,
				},
			})
		})
		mrrl.Events.Hit.Attach(mrlClosure)
		defer mrrl.Events.Hit.Detach(mrlClosure)
	}
	Plugin.LogInfof("%s started", PluginName)

	<-ctx.Done()

	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
