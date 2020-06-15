package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/stretchr/testify/assert"
)

func TestAttachment(t *testing.T) {
	transactionID := transaction.RandomID()
	payloadID := payload.RandomID()

	attachment := NewAttachment(transactionID, payloadID)

	assert.Equal(t, transactionID, attachment.TransactionID())
	assert.Equal(t, payloadID, attachment.PayloadID())

	clonedAttachment, consumedBytes, err := AttachmentFromBytes(attachment.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, AttachmentLength, consumedBytes)
	assert.Equal(t, transactionID, clonedAttachment.TransactionID())
	assert.Equal(t, payloadID, clonedAttachment.PayloadID())
}
