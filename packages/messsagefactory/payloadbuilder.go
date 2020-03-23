package messsagefactory

import "github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"

type PayloadBuilder interface {
	BuildPayload() *payload.Payload
}
