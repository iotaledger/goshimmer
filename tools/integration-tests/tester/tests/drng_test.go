package tests

import (
	"encoding/hex"
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
	time.Sleep(100 * time.Second)

	peers := len(drng.Peers())
	randomnessMap := make(map[string]int)

	// check for randomness on every peer
	for i := 0; i < 3; i++ {
		for _, peer := range drng.Peers() {
			resp, err := peer.GetRandomness()
			require.NoError(t, err)
			log.Println(resp)
			randomnessMap[hex.EncodeToString(resp.Randomness)]++
		}
		// wait for the next randomness
		time.Sleep(10 * time.Second)
	}

	// check that we got at least 3 different random values
	require.GreaterOrEqual(t, len(randomnessMap), 3)

	// check that each random values has been received by all the peers.
	for _, v := range randomnessMap {
		require.GreaterOrEqual(t, v, peers)
	}
}
