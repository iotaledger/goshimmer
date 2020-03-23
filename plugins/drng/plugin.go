package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

const name = "DRNG" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var Instance *drng.Instance

func configure(*node.Plugin) {
	Instance = drng.New()
	configureEvents()
}

func run(*node.Plugin) {}

func configureEvents() {
	tangle.Instance.Events.TransactionSolid.Attach(events.NewClosure(func(cachedTransaction *message.CachedTransaction, cachedTransactionMetadata *transactionmetadata.CachedTransactionMetadata) {
		cachedTransactionMetadata.Release()

		cachedTransaction.Consume(func(transaction *message.Transaction) {
			marshalUtil := marshalutil.New(transaction.GetPayload().Bytes())
			parsedPayload, err := payload.Parse(marshalUtil)
			if err != nil {
				//TODO: handle error
				return
			}
			if err := Instance.Dispatch(transaction.IssuerPublicKey(), transaction.IssuingTime(), parsedPayload); err != nil {
				//TODO: handle error
			}
		})
	}))

}
