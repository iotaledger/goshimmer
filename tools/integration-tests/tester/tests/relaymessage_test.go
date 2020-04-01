package tests

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
)

var f *framework.Framework

func TestMain(m *testing.M) {
	f = framework.New()

	// call the tests
	os.Exit(m.Run())
}

func TestRelayMessages(t *testing.T) {
	numMessages := 100
	hashes := make([]string, numMessages)

	// create messages on random peers
	for i := 0; i < numMessages; i++ {
		hash, err := f.RandomPeer().BroadcastData("Test")
		require.NoError(t, err)

		hashes[i] = hash
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// check for messages on every peer
	for _, peer := range f.Peers() {
		resp, err := peer.GetMessageByHash(hashes)
		require.NoError(t, err)

		// check for the count of messages
		assert.Equal(t, numMessages, len(resp.Messages))

		// check that all messages are present in response
	outer:
		for _, hash := range hashes {
			for _, msg := range resp.Messages {
				// if message found skip to next
				if msg.MessageId == hash {
					continue outer
				}
			}

			t.Errorf("MessageId=%s not found in peer %s.", hash, peer.String())
		}
	}
}
