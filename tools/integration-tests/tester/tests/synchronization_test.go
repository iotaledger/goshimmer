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
		id, sent := sendDataMessage(t, peer, data, i)

		idsMap[id] = sent
		ids[i] = id
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// make sure every peer got every message
	checkForMessageIds(t, n.Peers(), ids, idsMap)

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
		id, sent := sendDataMessage(t, peer, data, i)

		ids2[i] = id
		idsMap[id] = sent
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// check whether peer has synchronized ids (previous messages)
	checkForMessageIds(t, []*framework.Peer{newPeer}, ids, idsMap)

	// make sure every peer got every message
	checkForMessageIds(t, n.Peers(), ids2, idsMap)
}

type MessageSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

func sendDataMessage(t *testing.T, peer *framework.Peer, data []byte, number int) (string, MessageSent) {
	id, err := peer.Data(data)
	require.NoError(t, err)

	sent := MessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewData(data).Bytes(),
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return id, sent
}

func checkForMessageIds(t *testing.T, peers []*framework.Peer, ids []string, idsMap map[string]MessageSent) {
	for _, peer := range peers {
		resp, err := peer.FindMessageByID(ids)
		require.NoError(t, err)

		// check that all messages are present in response
		respIDs := make([]string, len(resp.Messages))
		for i, msg := range resp.Messages {
			respIDs[i] = msg.ID
		}
		assert.ElementsMatchf(t, ids, respIDs, "messages do not match sent in %s", peer.String())

		// check for general information
		for _, msg := range resp.Messages {
			msgSent := idsMap[msg.ID]

			assert.Equalf(t, msgSent.issuerPublicKey, msg.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Equalf(t, msgSent.data, msg.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Truef(t, msg.Metadata.Solid, "messageID=%s, issuer=%s not solid in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
		}
	}
}
