package tangle

import (
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

type Attachment struct {
	transactionId transaction.Id
	payloadId     payload.Id
}
