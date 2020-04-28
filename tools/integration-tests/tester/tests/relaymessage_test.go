package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelayMessages checks whether messages are actually relayed/gossiped through the network
// by checking the messages' existence on all nodes after a cool down.
func TestRelayMessages(t *testing.T) {
	n, err := f.CreateNetwork("TestRelayMessages", 6, 3)
	require.NoError(t, err)
	defer n.Shutdown()

	numMessages := 105
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
	time.Sleep(10 * time.Second)

	// check for messages on every peer
	for _, peer := range n.Peers() {
		resp, err := peer.FindMessageByID(ids)
		require.NoError(t, err)

		// check for the count of messages
		assert.Equal(t, numMessages, len(resp.Messages))

		// check that all messages are present in response
		idsSeen := make(map[string]bool)
		for _, id := range ids {
			idsSeen[id] = false
		}
		for _, msg := range resp.Messages {
			if _, ok := idsSeen[msg.ID]; !ok {
				t.Errorf("MessageId=%s found but not created. %s.", msg.ID, peer.String())
				continue
			}

			idsSeen[msg.ID] = true
		}
		for id, seen := range idsSeen {
			if !seen {
				t.Errorf("MessageId=%s not found in peer %s.", id, peer.String())
			}
		}
	}
}
