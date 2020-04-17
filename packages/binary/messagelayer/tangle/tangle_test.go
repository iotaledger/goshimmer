package tangle

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

func BenchmarkTangle_AttachMessage(b *testing.B) {
	dir, err := ioutil.TempDir("", b.Name())
	require.NoError(b, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	config.Node.Set(database.CFG_DIRECTORY, dir)

	tangle := New(database.GetBadgerInstance())
	if err := tangle.Prune(); err != nil {
		b.Error(err)

		return
	}

	testIdentity := identity.GenerateLocalIdentity()

	messageBytes := make([]*message.Message, b.N)
	for i := 0; i < b.N; i++ {
		messageBytes[i] = message.New(message.EmptyId, message.EmptyId, testIdentity, time.Now(), 0, payload.NewData([]byte("some data")))
		messageBytes[i].Bytes()
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tangle.AttachMessage(messageBytes[i])
	}

	tangle.Shutdown()
}

func TestTangle_AttachMessage(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	config.Node.Set(database.CFG_DIRECTORY, dir)

	messageTangle := New(database.GetBadgerInstance())
	if err := messageTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	messageTangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			fmt.Println("ATTACHED:", msg.Id())
		})
	}))

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *CachedMessageMetadata) {
		cachedMessageMetadata.Release()

		cachedMessage.Consume(func(msg *message.Message) {
			fmt.Println("SOLID:", msg.Id())
		})
	}))

	messageTangle.Events.MessageUnsolidifiable.Attach(events.NewClosure(func(messageId message.Id) {
		fmt.Println("UNSOLIDIFIABLE:", messageId)
	}))

	messageTangle.Events.MessageMissing.Attach(events.NewClosure(func(messageId message.Id) {
		fmt.Println("MISSING:", messageId)
	}))

	messageTangle.Events.MessageRemoved.Attach(events.NewClosure(func(messageId message.Id) {
		fmt.Println("REMOVED:", messageId)
	}))

	localIdentity1 := identity.GenerateLocalIdentity()
	localIdentity2 := identity.GenerateLocalIdentity()
	newMessageOne := message.New(message.EmptyId, message.EmptyId, localIdentity1, time.Now(), 0, payload.NewData([]byte("some data")))
	newMessageTwo := message.New(newMessageOne.Id(), newMessageOne.Id(), localIdentity2, time.Now(), 0, payload.NewData([]byte("some other data")))

	messageTangle.AttachMessage(newMessageTwo)

	time.Sleep(7 * time.Second)

	messageTangle.AttachMessage(newMessageOne)

	messageTangle.Shutdown()
}
