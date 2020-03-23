package tangle

import (
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// Attachment stores the information which transaction was attached by which transaction. We need this to perform
// reverse lookups for which Payloads contain which Transactions.
type Attachment struct {
	transactionId transaction.Id
	payloadId     payload.Id
}

// TransactionId returns the transaction id of this Attachment.
func (attachment *Attachment) TransactionId() transaction.Id {
	return attachment.transactionId
}

// PayloadId returns the payload id of this Attachment.
func (attachment *Attachment) PayloadId() payload.Id {
	return attachment.payloadId
}
