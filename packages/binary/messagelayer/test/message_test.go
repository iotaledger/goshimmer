package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/testutil"
)

func TestMessage_StorableObjectFromKey(t *testing.T) {
	key, err := message.NewId("2DYebCqnZ8PS5PyXBEvAvLB1fCF77Rn9RtofNHjEb2pSTujKi889d31FmguAs5DgL7YURw4GP2Y28JdJ7K4bjudG")
	if err != nil {
		panic(err)
	}

	messageFromKey, err, consumedBytes := message.StorableObjectFromKey(key.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, message.IdLength, consumedBytes)
	assert.Equal(t, key, messageFromKey.(*message.Message).Id())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	testMessage := testutil.MessageFactory(t).IssuePayload(payload.NewData([]byte("sth")))
	assert.Equal(t, true, testMessage.VerifySignature())

	restoredMessage, err, _ := message.FromBytes(testMessage.Bytes())
	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.Id(), restoredMessage.Id())
		assert.Equal(t, testMessage.TrunkMessageId(), restoredMessage.TrunkMessageId())
		assert.Equal(t, testMessage.BranchMessageId(), restoredMessage.BranchMessageId())
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
	}
}
