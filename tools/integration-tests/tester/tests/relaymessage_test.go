package tests

import (
	"os"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		req := BroadcastDataRequest{Data: "Test"}
		resp := new(BroadcastDataResponse)
		err := f.HttpPost(f.RandomPeer(), "broadcastData", req, resp)
		require.NoError(t, err)

		hashes[i] = resp.Hash
	}

	// wait for messages to be gossiped
	time.Sleep(5 * time.Second)

	// check for messages on every peer
	for _, peer := range f.Peers() {
		req := GetMessageByHashRequest{Hashes: hashes}
		resp := new(GetMessageByHashResponse)
		err := f.HttpPost(peer, "getMessageByHash", req, resp)
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

// TODO: there should be a nice way to handle all those API endpoints
type BroadcastDataResponse struct {
	Hash  string `json:"hash,omitempty"`
	Error string `json:"error,omitempty"`
}

type BroadcastDataRequest struct {
	Data string `json:"data"`
}

type GetMessageByHashResponse struct {
	Messages []Message `json:"messages,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type GetMessageByHashRequest struct {
	Hashes []string `json:"hashes"`
}

type Message struct {
	MessageId           string `json:"messageId,omitempty"`
	TrunkTransactionId  string `json:"trunkTransactionId,omitempty"`
	BranchTransactionId string `json:"branchTransactionId,omitempty"`
	IssuerPublicKey     string `json:"issuerPublicKey,omitempty"`
	IssuingTime         string `json:"issuingTime,omitempty"`
	SequenceNumber      uint64 `json:"sequenceNumber,omitempty"`
	Payload             string `json:"payload,omitempty"`
	Signature           string `json:"signature,omitempty"`
}
