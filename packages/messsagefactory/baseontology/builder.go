package baseontology

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload/data"
	"github.com/iotaledger/goshimmer/packages/messsagefactory"
)

type Builder struct {
}

func (b *Builder) BuildPayload(raw []byte) *payload.Payload {
	var dataPayload payload.Payload = data.New(raw)

	messsagefactory.Events.PayloadConstructed.Trigger(&dataPayload)

	return &dataPayload
}
