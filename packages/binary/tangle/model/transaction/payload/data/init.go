package data

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
)

func init() {
	payload.RegisterType(Type, GenericPayloadUnmarshalerFactory(Type))
}
