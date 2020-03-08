package transactionfactory

import "github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"

type PayloadBuilder interface {
	BuildPayload() *payload.Payload
}
