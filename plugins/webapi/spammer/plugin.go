package spammer

import (
	"github.com/iotaledger/goshimmer/packages/binary/spammer"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/webapi"

	"github.com/iotaledger/hive.go/node"
)

var transactionSpammer *spammer.Spammer

var PLUGIN = node.NewPlugin("Spammer", node.Enabled, configure)

func configure(plugin *node.Plugin) {
	transactionSpammer = spammer.New(tangle.Instance, tangle.TipSelector)

	webapi.Server.GET("spammer", handleRequest)
}
