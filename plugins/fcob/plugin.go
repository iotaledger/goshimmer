package fcob

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/fpc"
)

var PLUGIN = node.NewPlugin("FCOB", configure, run)

var INSTANCE *Fcob

func configure(plugin *node.Plugin) {
	fpc.
	INSTANCE = &Fcob{fpc: fpc}
	
	gossip.Events.ReceiveTransaction.Attach(
		events.NewClosure(func(transaction *transaction.Transaction) {
			INSTANCE.ReceiveTransaction(transaction)
		}),
	)
}

func run(plugin *node.Plugin) {}
