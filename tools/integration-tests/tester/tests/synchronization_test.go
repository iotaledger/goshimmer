package tests

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNodeSynchronization checks whether messages are synchronized by a peer that joined the network later
// and initially missed some messages.
func TestNodeSynchronization(t *testing.T) {
	n, err := f.CreateNetwork("TestNodeSynchronization", 4, 2)
	require.NoError(t, err)
	defer n.Shutdown()

	numMessages := 100
	ids := make([]string, numMessages)

	data := []byte("Test")

	// create messages on random peers
	for i := 0; i < numMessages; i++ {
		peer := n.RandomPeer()
		id, err := peer.Data(data)
		require.NoError(t, err)

		ids[i] = id
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// make sure every peer got every message
	checkForMessageIds(t, n.Peers(), numMessages, ids)

	// spawn peer without knowledge of previous messages
	newPeer, err := n.CreatePeer()
	require.NoError(t, err)
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	ids2 := make([]string, numMessages)

	// create messages on random peers
	for i := 0; i < numMessages; i++ {
		peer := n.RandomPeer()
		id, err := peer.Data(data)
		require.NoError(t, err)

		ids2[i] = id
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// check whether peer has synchronized ids (previous messages)
	checkForMessageIds(t, []*framework.Peer{newPeer}, numMessages, ids)

	// make sure every peer got every message
	checkForMessageIds(t, n.Peers(), numMessages, ids2)
}

func checkForMessageIds(t *testing.T, peers []*framework.Peer, numMessages int, ids []string) {
	for _, peer := range peers {
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
