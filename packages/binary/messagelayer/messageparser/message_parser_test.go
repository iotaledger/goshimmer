package messageparser

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func BenchmarkMessageParser_ParseBytesSame(b *testing.B) {
	localIdentity := identity.GenerateLocalIdentity()
	txBytes := message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("Test"))).Bytes()
	txParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txParser.Parse(txBytes, nil)
	}

	txParser.Shutdown()
}

func BenchmarkMessageParser_ParseBytesDifferent(b *testing.B) {
	messageBytes := make([][]byte, b.N)
	localIdentity := identity.GenerateLocalIdentity()
	for i := 0; i < b.N; i++ {
		messageBytes[i] = message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("Test"+strconv.Itoa(i)))).Bytes()
	}

	txParser := New()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txParser.Parse(messageBytes[i], nil)
	}

	txParser.Shutdown()
}

func TestMessageParser_ParseMessage(t *testing.T) {
	localIdentity := identity.GenerateLocalIdentity()
	tx := message.New(message.EmptyId, message.EmptyId, localIdentity, time.Now(), 0, payload.NewData([]byte("Test")))

	txParser := New()
	txParser.Parse(tx.Bytes(), nil)

	txParser.Events.MessageParsed.Attach(events.NewClosure(func(tx *message.Message) {
		fmt.Println("PARSED!!!")
	}))

	txParser.Shutdown()
}
