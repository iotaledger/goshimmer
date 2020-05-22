package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func TestMessage_StorableObjectFromKey(t *testing.T) {
	key, err := message.NewId("2DYebCqnZ8PS5PyXBEvAvLB1fCF77Rn9RtofNHjEb2pSTujKi889d31FmguAs5DgL7YURw4GP2Y28JdJ7K4bjudG")
	if err != nil {
		panic(err)
	}

	messageFromKey, consumedBytes, err := message.StorableObjectFromKey(key.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, message.IdLength, consumedBytes)
	assert.Equal(t, key, messageFromKey.(*message.Message).Id())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	msgFactory := messagefactory.New(mapdb.NewMapDB(), identity.GenerateLocalIdentity(), tipselector.New(), []byte(messagelayer.DBSequenceNumber))
	defer msgFactory.Shutdown()

	testMessage := msgFactory.IssuePayload(payload.NewData([]byte("sth")))
	assert.Equal(t, true, testMessage.VerifySignature())

	fmt.Print(testMessage)

	restoredMessage, err, _ := message.FromBytes(testMessage.Bytes())
	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.Id(), restoredMessage.Id())
		assert.Equal(t, testMessage.TrunkId(), restoredMessage.TrunkId())
		assert.Equal(t, testMessage.BranchId(), restoredMessage.BranchId())
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
		assert.Equal(t, testMessage.IssuingTime().Round(time.Second), restoredMessage.IssuingTime().Round(time.Second))
		assert.Equal(t, testMessage.SequenceNumber(), restoredMessage.SequenceNumber())
		assert.Equal(t, testMessage.Signature(), restoredMessage.Signature())
		assert.Equal(t, true, restoredMessage.VerifySignature())
	}
}
