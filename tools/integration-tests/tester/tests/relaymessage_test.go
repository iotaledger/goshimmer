package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func TestRelayMessages(t *testing.T) {
	numMessages := 100
	ids := make([]string, numMessages)

	data := payload.NewData([]byte("Test")).Bytes()

	// create messages on random peers
	for i := 0; i < numMessages; i++ {
		id, err := f.RandomPeer().BroadcastData(data)
		require.NoError(t, err)

		ids[i] = id
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// check for messages on every peer
	for _, peer := range f.Peers() {
		resp, err := peer.FindMessageById(ids)
		require.NoError(t, err)

		// check for the count of messages
		assert.Equal(t, numMessages, len(resp.Messages))

		// check that all messages are present in response
	outer:
		for _, id := range ids {
			for _, msg := range resp.Messages {
				// if message found skip to next
				if msg.Id == id {
					continue outer
				}
			}

			t.Errorf("MessageId=%s not found in peer %s.", id, peer.String())
		}
	}
}
