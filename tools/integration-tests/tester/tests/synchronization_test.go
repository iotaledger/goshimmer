package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
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
	idsMap := make(map[string]MessageSent, numMessages)
	ids := make([]string, numMessages)

	// create messages on random peers
	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test%d", i))

		peer := n.RandomPeer()
		id, err := peer.Data(data)
		require.NoError(t, err)

		idsMap[id] = MessageSent{
			number: i,
			id:     id,
			// save payload to be able to compare API response
			data:            payload.NewData(data).Bytes(),
			issuerPublicKey: peer.Identity.PublicKey().String(),
		}
		ids[i] = id
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// make sure every peer got every message
	checkForMessageIds(t, n.Peers(), numMessages, ids, idsMap)

	// spawn peer without knowledge of previous messages
	newPeer, err := n.CreatePeer()
	require.NoError(t, err)
	err = n.WaitForAutopeering(3)
	require.NoError(t, err)

	ids2 := make([]string, numMessages)

	// create messages on random peers
	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test%d", i))

		peer := n.RandomPeer()
		id, err := peer.Data(data)
		require.NoError(t, err)

		ids2[i] = id
		idsMap[id] = MessageSent{
			number: i,
			id:     id,
			// save payload to be able to compare API response
			data:            payload.NewData(data).Bytes(),
			issuerPublicKey: peer.Identity.PublicKey().String(),
		}
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// check whether peer has synchronized ids (previous messages)
	checkForMessageIds(t, []*framework.Peer{newPeer}, numMessages, ids, idsMap)

	// make sure every peer got every message
	checkForMessageIds(t, n.Peers(), numMessages, ids2, idsMap)
}

type MessageSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

func checkForMessageIds(t *testing.T, peers []*framework.Peer, numMessages int, ids []string, idsMap map[string]MessageSent) {
	for _, peer := range peers {
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
				t.Errorf("MessageId=%s, issuer=%s not found in peer %s.", id, idsMap[id].issuerPublicKey, peer.String())
			}
		}

		// check for general information
		for _, msg := range resp.Messages {
			msgSent := idsMap[msg.ID]

			assert.Equal(t, msgSent.issuerPublicKey, msg.IssuerPublicKey, "MessageId=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Equal(t, msgSent.data, msg.Payload, "MessageId=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.True(t, msg.Metadata.Solid, "MessageId=%s, issuer=%s not solid in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
		}
	}
}
