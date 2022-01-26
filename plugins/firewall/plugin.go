package firewall

import (
	"context"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
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

	GossipMgr      *gossip.Manager
	AutopeeringMgr *selection.Protocol `optional:"true"`
	Firewall       *firewall.Firewall
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Attach(events.NewClosure(func(_ *node.Plugin, container *dig.Container) {
		if err := container.Provide(createFirewall); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func createFirewall() *firewall.Firewall {
	return firewall.NewFirewall(deps.GossipMgr, deps.AutopeeringMgr, Plugin.Logger())
}

func configure(_ *node.Plugin) {
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityFirewall); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func start(ctx context.Context) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")
	mrlClosure := events.NewClosure(func(p *peer.Peer, rl *ratelimiter.RateLimit) {
		deps.Firewall.OnFaultyPeer(p, &firewall.FaultinessDetails{
			Reason: "Messages rate limit hit",
			Info: map[string]interface{}{
				"rateLimit": rl,
			},
		})
	})

	if mrl := deps.GossipMgr.MessagesRateLimiter(); mrl != nil {
		mrl.HitEvent().Attach(mrlClosure)
		defer mrl.HitEvent().Detach(mrlClosure)
	}
	Plugin.LogInfof("%s started", PluginName)

	<-ctx.Done()

	Plugin.LogInfo("Stopping " + PluginName + " ...")
}
