package faucet

import (
	"github.com/iotaledger/goshimmer/packages/binary/faucet"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/tangle"
)

const name = "Faucet" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	configureEvents()
}

func configureEvents() {
	tangle.TransactionParser.Events.TransactionParsed.Attach(events.NewClosure(func(transaction *transaction.Transaction, peer *peer.Peer) {
		if faucet.IsFaucetReq(transaction) {
			faucet.SendFunds(transaction)
		}
	}))
}

func run(*node.Plugin) {}
