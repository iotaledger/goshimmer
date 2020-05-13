package tests

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DataMessageSent defines a struct to identify from which issuer a data message was sent.
type DataMessageSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

// sendDataMessagesOnRandomPeer sends data messages on a random peer and saves the sent message to a map.
func sendDataMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, numMessages int, idsMap ...map[string]DataMessageSent) map[string]DataMessageSent {
	var ids map[string]DataMessageSent
	if len(idsMap) > 0 {
		ids = idsMap[0]
	} else {
		ids = make(map[string]DataMessageSent, numMessages)
	}

	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test%d", i))

		peer := peers[rand.Intn(len(peers))]
		id, sent := sendDataMessage(t, peer, data, i)

		ids[id] = sent
	}

	return ids
}

// sendDataMessage sends a data message on a given peer and returns the id and a DataMessageSent struct.
func sendDataMessage(t *testing.T, peer *framework.Peer, data []byte, number int) (string, DataMessageSent) {
	id, err := peer.Data(data)
	require.NoErrorf(t, err, "Could not send message on %s", peer.String())

	sent := DataMessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewData(data).Bytes(),
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return id, sent
}

// checkForMessageIds performs checks to make sure that all peers received all given messages defined in ids.
func checkForMessageIds(t *testing.T, peers []*framework.Peer, ids map[string]DataMessageSent, checkSynchronized bool) {
	var idsSlice []string
	for id := range ids {
		idsSlice = append(idsSlice, id)
	}

	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			require.True(t, info.Synced)
		}

		resp, err := peer.FindMessageByID(idsSlice)
		require.NoError(t, err)

		// check that all messages are present in response
		respIDs := make([]string, len(resp.Messages))
		for i, msg := range resp.Messages {
			respIDs[i] = msg.ID
		}
		assert.ElementsMatchf(t, idsSlice, respIDs, "messages do not match sent in %s", peer.String())

		// check for general information
		for _, msg := range resp.Messages {
			msgSent := ids[msg.ID]

			assert.Equalf(t, msgSent.issuerPublicKey, msg.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Equalf(t, msgSent.data, msg.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Truef(t, msg.Metadata.Solid, "messageID=%s, issuer=%s not solid in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
		}
	}
}


// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n *framework.Network) {
	err := n.Shutdown()
	require.NoError(t, err)
}