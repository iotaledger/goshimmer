package spammer

import (
	"github.com/iotaledger/hive.go/daemon"

	"github.com/iotaledger/goshimmer/packages/binary/spammer"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"

	"github.com/iotaledger/hive.go/node"
)

var messageSpammer *spammer.Spammer

var PLUGIN = node.NewPlugin("Spammer", node.Disabled, configure, run)

func configure(plugin *node.Plugin) {
	messageSpammer = spammer.New(messagelayer.MessageFactory)
	webapi.Server.GET("spammer", handleRequest)
}

func run(*node.Plugin) {
	_ = daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal

		messageSpammer.Shutdown()
	}, shutdown.ShutdownPrioritySpammer)
}
