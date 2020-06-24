package faucet

import (
	faucet "github.com/iotaledger/goshimmer/dapps/faucet/packages"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const name = "Faucet" // name of the plugin

var (
	// App is the "plugin" instance of the faucet application.
	App = node.NewPlugin(name, node.Disabled, configure, run)
	log *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)
	faucet.ConfigureFaucet()
	configureEvents()
}

func configureEvents() {
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *tangle.CachedMessageMetadata) {
		defer cachedTransaction.Release()
		cachedTransactionMetadata.Release()

		msg := cachedTransaction.Unwrap()
		if msg == nil {
			log.Errorf("Fail to unwrap cachedTransaction")
			return
		}

		if !faucet.IsFaucetReq(msg) {
			return
		}
		log.Info("got a faucet request")

		// send funds
		txID, err := faucet.SendFunds(msg)
		if err != nil {
			log.Errorf("Fail to send funds on faucet request")
			return
		}
		log.Info("send funds on faucet, txID: ", txID)
	}))
}

func run(*node.Plugin) {}
