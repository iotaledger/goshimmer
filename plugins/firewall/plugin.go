package firewall

import (
	"context"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

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

	if mrl := deps.GossipMgr.MessagesRateLimiter(); mrl != nil {
		mrlClosure := events.NewClosure(func(p *peer.Peer, rl *ratelimiter.RateLimit) {
			deps.Firewall.HandleFaultyPeer(p.ID(), &firewall.FaultinessDetails{
				Reason: "Messages rate limit hit",
				Info: map[string]interface{}{
					"rateLimit": rl,
				},
			})
		})
		mrl.HitEvent().Attach(mrlClosure)
		defer mrl.HitEvent().Detach(mrlClosure)
	}
	if mrrl := deps.GossipMgr.MessageRequestsRateLimiter(); mrrl != nil {
		mrlClosure := events.NewClosure(func(p *peer.Peer, rl *ratelimiter.RateLimit) {
			deps.Firewall.HandleFaultyPeer(p.ID(), &firewall.FaultinessDetails{
				Reason: "Message requests rate limit hit",
				Info: map[string]interface{}{
					"rateLimit": rl,
				},
			})
		})
		mrrl.HitEvent().Attach(mrlClosure)
		defer mrrl.HitEvent().Detach(mrlClosure)
	}
	Plugin.LogInfof("%s started", PluginName)

	<-ctx.Done()

	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
