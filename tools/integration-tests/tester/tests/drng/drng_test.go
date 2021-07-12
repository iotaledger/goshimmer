package drng

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestDRNG checks whether drng messages are actually relayed/gossiped through the network
// by checking the messages' existence on all nodes after a cool down.
func TestDRNG(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateDRNGNetwork(ctx, t.Name(), 5, 6)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// wait for randomness generation to be started
	log.Println("Waiting for randomness generation to be started...")
	require.Eventually(t,
		func() bool { return getRandomness(t, n.Peers()[0]).Round > 0 },
		tests.Timeout, tests.Tick)
	log.Println("Waiting for randomness generation to be started... done")

	firstRound := getRandomness(t, n.Peers()[0]).Round
	const numChecks = 3
	for i := 0; i < numChecks; i++ {
		round := firstRound + uint64(i)
		// eventually all peers should be in the same round
		log.Printf("Waiting for all peers to receive round %d...", round)
		require.Eventually(t,
			func() bool {
				for _, peer := range n.Peers() {
					if getRandomness(t, peer).Round != round {
						return false
					}
				}
				return true
			},
			20*time.Second, tests.Tick)
		log.Println("Waiting for all peers to receive round... done")
	}
}

func getRandomness(t *testing.T, node *framework.Node) jsonmodels.Randomness {
	resp, err := node.GetRandomness()
	require.NoError(t, err)

	id := uint32(node.Config().DRNG.Custom.InstanceID)
	for i := range resp.Randomness {
		if resp.Randomness[i].InstanceID == id {
			return resp.Randomness[i]
		}
	}
	panic("invalid InstanceID")
}
