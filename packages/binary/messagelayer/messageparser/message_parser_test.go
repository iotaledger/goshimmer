package messageparser

import (
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func BenchmarkMessageParser_ParseBytesSame(b *testing.B) {
	localIdentity := identity.GenerateLocalIdentity()
	msgBytes := message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("Test"))).Bytes()
	msgParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(msgBytes, nil)
	}

	msgParser.Shutdown()
}

func BenchmarkMessageParser_ParseBytesDifferent(b *testing.B) {
	messageBytes := make([][]byte, b.N)
	localIdentity := identity.GenerateLocalIdentity()
	for i := 0; i < b.N; i++ {
		messageBytes[i] = message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("Test"+strconv.Itoa(i)))).Bytes()
	}

	msgParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(messageBytes[i], nil)
	}

	msgParser.Shutdown()
}

func TestMessageParser_ParseMessage(t *testing.T) {
	localIdentity := identity.GenerateLocalIdentity()
	msg := message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("Test")))

	msgParser := New()
	msgParser.Parse(msg.Bytes(), nil)

	msgParser.Events.MessageParsed.Attach(events.NewClosure(func(msg *message.Message) {
		log.Infof("parsed message")
	}))

	msgParser.Shutdown()
}
