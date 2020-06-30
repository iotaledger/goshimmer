package messageparser

import (
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func BenchmarkMessageParser_ParseBytesSame(b *testing.B) {
	msgBytes := newTestMessage("Test").Bytes()
	msgParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(msgBytes, nil)
	}

	msgParser.Shutdown()
}

func BenchmarkMessageParser_ParseBytesDifferent(b *testing.B) {
	messageBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestMessage("Test" + strconv.Itoa(i)).Bytes()
	}

	msgParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(messageBytes[i], nil)
	}

	msgParser.Shutdown()
}

func TestMessageParser_ParseMessage(t *testing.T) {
	msg := newTestMessage("Test")

	msgParser := New()
	msgParser.Parse(msg.Bytes(), nil)

	msgParser.Events.MessageParsed.Attach(events.NewClosure(func(msg *message.Message) {
		log.Infof("parsed message")
	}))

	msgParser.Shutdown()
}

func newTestMessage(payloadString string) *message.Message {
	return message.New(message.EmptyId, message.EmptyId, time.Now(), ed25519.PublicKey{}, 0, payload.NewData([]byte(payloadString)), 0, ed25519.Signature{})
}
