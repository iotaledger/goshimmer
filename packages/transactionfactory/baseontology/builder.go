package baseontology

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
	"github.com/iotaledger/goshimmer/packages/transactionfactory"
)

type Builder struct {
}

func (b *Builder) BuildPayload(raw []byte) *payload.Payload {
	var dataPayload payload.Payload = data.New(raw)

	transactionfactory.Events.PayloadConstructed.Trigger(&dataPayload)

	return &dataPayload
}
