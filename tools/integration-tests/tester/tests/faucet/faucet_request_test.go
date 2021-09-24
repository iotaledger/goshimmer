package faucet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestFaucetRequest sends funds by faucet request.
func TestFaucetRequest(t *testing.T) {
	const (
		numPeers    = 4
		numRequests = 2
	)

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), numPeers, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Activity:    true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peers := n.Peers()[0], n.Peers()[1:]

	// wait for the faucet to prepare initial outputs
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet)

	// each non-faucet peer issues numRequests requests
	for _, peer := range peers {
		for idx := 0; idx < numRequests; idx++ {
			tests.SendFaucetRequest(t, peer, peer.Address(idx))
		}
	}

	// wait for all peers to register their new balances
	for _, peer := range peers {
		for idx := 0; idx < numRequests; idx++ {
			require.Eventuallyf(t, func() bool {
				balance := tests.Balance(t, peer, peer.Address(idx), ledgerstate.ColorIOTA)
				return balance == uint64(faucet.Config().TokensPerRequest)
			}, tests.Timeout, tests.Tick,
				"peer %s did not register its requested funds on address %s", peer, peer.Address(idx).Base58())
		}
	}
}
