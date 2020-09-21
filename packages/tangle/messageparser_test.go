package tangle

import (
	"strconv"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/gommon/log"
)

func BenchmarkMessageParser_ParseBytesSame(b *testing.B) {
	msgBytes := newTestDataMessage("Test").Bytes()
	msgParser := NewMessageParser()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(msgBytes, nil)
	}
}

func BenchmarkMessageParser_ParseBytesDifferent(b *testing.B) {
	messageBytes := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = newTestDataMessage("Test" + strconv.Itoa(i)).Bytes()
	}

	msgParser := NewMessageParser()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgParser.Parse(messageBytes[i], nil)
	}
}

func TestMessageParser_ParseMessage(t *testing.T) {
	msg := newTestDataMessage("Test")

	msgParser := NewMessageParser()
	msgParser.Parse(msg.Bytes(), nil)

	msgParser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		log.Infof("parsed message")
	}))
}
