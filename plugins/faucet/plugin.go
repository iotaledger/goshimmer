package faucet

import (
	"github.com/iotaledger/goshimmer/packages/binary/faucet"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const name = "Faucet" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	configureEvents()
}

func configureEvents() {
	messagelayer.Tangle.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *tangle.CachedMessageMetadata) {
		if msg := cachedTransaction.Unwrap(); msg != nil {
			if faucet.IsFaucetReq(msg) {
				faucet.SendFunds(msg)
			}
		} else {
			log.Errorf("Fail to unwrap cachedTransaction")
		}
	}))
}

func run(*node.Plugin) {}
