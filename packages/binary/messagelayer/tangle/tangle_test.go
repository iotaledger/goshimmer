package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func BenchmarkTangle_AttachMessage(b *testing.B) {
	tangle := New(mapdb.NewMapDB())
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	messageBytes := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestMessage("some data")
		messageBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.AttachMessage(messageBytes[i])
	}

	tangle.Shutdown()
}

func TestTangle_AttachMessage(t *testing.T) {
	messageTangle := New(mapdb.NewMapDB())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *CachedMessageEvent) {
		cachedMessage.MessageMetadata.Release()

		cachedMessage.Message.Consume(func(msg *message.Message) {
			fmt.Println("ATTACHED:", msg.ID())
		})
	}))

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *CachedMessageEvent) {
		cachedMessage.MessageMetadata.Release()

		cachedMessage.Message.Consume(func(msg *message.Message) {
			fmt.Println("SOLID:", msg.ID())
		})
	}))

	messageTangle.Events.MessageUnsolidifiable.Attach(events.NewClosure(func(messageId message.ID) {
		fmt.Println("UNSOLIDIFIABLE:", messageId)
	}))

	messageTangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId message.ID) {
		fmt.Println("MISSING:", messageId)
	}))

	messageTangle.Events.MessageRemoved.Attach(events.NewClosure(func(messageId message.ID) {
		fmt.Println("REMOVED:", messageId)
	}))

	newMessageOne := newTestMessage("some data")
	newMessageTwo := newTestMessage("some other data")

	messageTangle.AttachMessage(newMessageTwo)

	time.Sleep(7 * time.Second)

	messageTangle.AttachMessage(newMessageOne)

	messageTangle.Shutdown()
}

func newTestMessage(payloadString string) *message.Message {
	return message.New(message.EmptyID, message.EmptyID, time.Now(), ed25519.PublicKey{}, 0, payload.NewData([]byte(payloadString)), 0, ed25519.Signature{})
}
