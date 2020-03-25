package message

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload/data"
)

func init() {
	payload.SetGenericUnmarshalerFactory(data.GenericPayloadUnmarshalerFactory)
}
