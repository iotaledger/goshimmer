package tangle

import (
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/gommon/log"
)

func BenchmarkMessageParser_ParseBytesSame(b *testing.B) {
	msgBytes := newTestMessage("Test").Bytes()
	msgParser := NewParser()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(msgBytes, nil)
	}
}

func BenchmarkMessageParser_ParseBytesDifferent(b *testing.B) {
	messageBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestMessage("Test" + strconv.Itoa(i)).Bytes()
	}

	msgParser := NewParser()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(messageBytes[i], nil)
	}
}

func TestMessageParser_ParseMessage(t *testing.T) {
	msg := newTestMessage("Test")

	msgParser := NewParser()
	msgParser.Parse(msg.Bytes(), nil)

	msgParser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		log.Infof("parsed message")
	}))
}

func newTestMessage(payloadString string) *Message {
	return NewMessage(EmptyMessageID, EmptyMessageID, time.Now(), ed25519.PublicKey{}, 0, NewDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}
