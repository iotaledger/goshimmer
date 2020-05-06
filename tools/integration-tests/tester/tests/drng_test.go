package tests

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDRNG checks whether drng messages are actually relayed/gossiped through the network
// by checking the messages' existence on all nodes after a cool down.
func TestDRNG(t *testing.T) {
	drng, err := f.CreateDRNGNetwork("TestDRNG", 5, 8)
	require.NoError(t, err)
	defer drng.Shutdown()

	// wait for randomness generation to be started
	log.Printf("Waiting for randomness generation to be started...\n")
	time.Sleep(70 * time.Second)

	peers := len(drng.Peers())

	// check for randomness on every peer
	for i := 0; i < 3; i++ {
		randomness := make([][]byte, peers)
		for j, peer := range drng.Peers() {
			resp, err := peer.GetRandomness()
			require.NoError(t, err)
			log.Println(resp)
			randomness[j] = resp.Randomness
		}
		for i := 1; i < peers; i++ {
			require.Equal(t, randomness[0], randomness[i])
		}
		time.Sleep(10 * time.Second)
	}
}
