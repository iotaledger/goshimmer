package firewall

import (
	"context"

	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/firewall"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/ratelimiter"
	"github.com/iotaledger/goshimmer/packages/shutdown"
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
}

type firewallDeps struct {
	dig.In
	AutopeeringMgr *selection.Protocol `optional:"true"`
	GossipMgr      *gossip.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
		if err := event.Container.Provide(createFirewall); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func createFirewall(fDeps firewallDeps) *firewall.Firewall {
	f, err := firewall.NewFirewall(fDeps.GossipMgr, fDeps.AutopeeringMgr, Plugin.Logger())
	if err != nil {
		Plugin.LogFatalf("Couldn't initialize firewall instance: %+v", err)
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
