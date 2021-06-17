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
	n, err := f.CreateDRNGNetwork(ctx, t.Name(), 5, 8)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// wait for randomness generation to be started
	log.Println("Waiting for randomness generation to be started...")
	require.Eventually(t,
		func() bool { return getRandomness(t, n.Peers()[0]).Round > 0 },
		tests.WaitForDeadline(t), tests.Tick)
	log.Println("Waiting for randomness generation to be started... done")

	firstRound := getRandomness(t, n.Peers()[0]).Round // TODO: Why doesn't this start with round=1?
	numChecks := 3

	// group the tests, to be able to clean up after all parallel tests have finished
	t.Run("round", func(t *testing.T) {
		for _, peer := range n.Peers() {
			p := peer // capture range variable
			t.Run(p.Name(), func(t *testing.T) {
				t.Parallel() // query all peers in parallel

				// check that we receive numChecks successive rounds of randomness
				for i := 0; i <= numChecks; i++ {
					round := firstRound + uint64(i)
					require.Eventuallyf(t,
						func() bool { return getRandomness(t, p).Round == round },
						20*time.Second, tests.Tick,
						"peer %s did not receive round %d", p, round)
					t.Logf("%+v", getRandomness(t, p))
				}
			})
		}
	})
}

func getRandomness(t *testing.T, node *framework.Node) jsonmodels.Randomness {
	resp, err := node.GetRandomness()
	require.NoError(t, err)

	id := uint32(node.Config().DRNG.Custom.InstanceId)
	for i := range resp.Randomness {
		if resp.Randomness[i].InstanceID == id {
			return resp.Randomness[i]
		}
	}
	panic("invalid InstanceID")
}
