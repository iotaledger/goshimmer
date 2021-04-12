package drng

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

var (
	errWrongRound = fmt.Errorf("wrong round")
)

// TestDRNG checks whether drng messages are actually relayed/gossiped through the network
// by checking the messages' existence on all nodes after a cool down.
func TestDRNG(t *testing.T) {
	var wg sync.WaitGroup

	drng, err := f.CreateDRNGNetwork("TestDRNG", 5, 8, 3)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, drng)

	// wait for randomness generation to be started
	log.Printf("Waiting for randomness generation to be started...\n")

	resp, firstRound, err := getFirstRound(t, drng.Peers()[0], 200)
	if err != nil {
		t.Log(resp)
	}
	require.NoError(t, err)

	log.Printf("Waiting for randomness generation to be started... done\n")

	ticker := time.NewTimer(0)
	defer ticker.Stop()

	numChecks := 3
	i := 0
	for {
		select {
		case <-ticker.C:
			ticker.Reset(10 * time.Second)

			// check for randomness on every peer
			// ignore last peer as that is only sync beacon node and does not have drng information
			for _, peer := range drng.Peers()[:len(drng.Peers())-1] {
				wg.Add(1)
				go func(peer *framework.Peer) {
					defer wg.Done()
					s, err := waitForRound(t, peer, firstRound+uint64(i), 8)
					require.NoError(t, err, peer.ID().String(), s)
					t.Log(peer.ID().String(), s)
				}(peer)
			}

			wg.Wait()
			i++

			if i == numChecks {
				return
			}
		}
	}
}

func getFirstRound(t *testing.T, peer *framework.Peer, maxAttempts int) (string, uint64, error) {
	var b []byte
	for i := 0; i < maxAttempts; i++ {
		resp, err := peer.GetRandomness()
		require.NoError(t, err)
		b, err = json.MarshalIndent(resp, "", " ")
		if err == nil && resp.Randomness[0].Round > 0 {
			return string(b), resp.Randomness[0].Round, nil
		}
		time.Sleep(1 * time.Second)
	}
	return string(b), 0, errWrongRound
}

func waitForRound(t *testing.T, peer *framework.Peer, round uint64, maxAttempts int) (string, error) {
	var b []byte
	for i := 0; i < maxAttempts; i++ {
		resp, err := peer.GetRandomness()
		require.NoError(t, err)
		b, _ = json.MarshalIndent(resp, "", " ")
		if resp.Randomness[0].Round == round {
			return string(b), nil
		}
		time.Sleep(1 * time.Second)
	}
	return string(b), errWrongRound
}
