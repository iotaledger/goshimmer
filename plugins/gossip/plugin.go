package gossip

import (
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
)

var PLUGIN = node.NewPlugin("Gossip", node.Enabled, configure, run)
var log = logger.NewLogger("Gossip")

var (
	debugLevel = "debug"
	close      = make(chan struct{}, 1)
)

func configure(plugin *node.Plugin) {
	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		close <- struct{}{}
	}))
	configureGossip()
	configureEvents()
}

func run(plugin *node.Plugin) {
}
