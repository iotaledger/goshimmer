package data

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
)

func init() {
	payload.RegisterType(Type, GenericPayloadUnmarshalerFactory(Type))
}
