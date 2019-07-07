package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("Bundle Processor", configure)

func configure(plugin *node.Plugin) {
	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		if tx.IsHead() {
			if _, err := ProcessSolidBundleHead(tx); err != nil {
				plugin.LogFailure(err.Error())
			}
		}
	}))
}
