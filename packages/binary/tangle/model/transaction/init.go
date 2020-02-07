package transaction

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
)

func init() {
	payload.SetGenericUnmarshalerFactory(data.GenericPayloadUnmarshalerFactory)
}
