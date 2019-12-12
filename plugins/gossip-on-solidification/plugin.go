package gossip_on_solidification

import (
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Gossip On Solidification", node.Enabled, func(plugin *node.Plugin) {
	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		gossip.SendTransaction(tx.MetaTransaction)
	}))
})
