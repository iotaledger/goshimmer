package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/stretchr/testify/assert"
)

func TestAttachment(t *testing.T) {
	transactionID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)
	// TODO: maybe make this public?
	messageID := randomMessageID()

	attachment := NewAttachment(transactionID, messageID)

	assert.Equal(t, transactionID, attachment.TransactionID())
	assert.Equal(t, messageID, attachment.MessageID())

	clonedAttachment, consumedBytes, err := AttachmentFromBytes(attachment.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, AttachmentLength, consumedBytes)
	assert.Equal(t, transactionID, clonedAttachment.TransactionID())
	assert.Equal(t, messageID, clonedAttachment.MessageID())
}
