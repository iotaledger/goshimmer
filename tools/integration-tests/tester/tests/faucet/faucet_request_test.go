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
		numPeers    = 5
		numRequests = 2
	)

	// TODO: we have numPeers*numRequests < preparedOutputCount; should we increase this?
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), numPeers, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// each non-faucet peer issues numRequests requests
	// TODO: can the faucet request funds for itself?
	for _, peer := range n.Peers()[1:] {
		for idx := 0; idx < numRequests; idx++ {
			tests.SendFaucetRequest(t, peer, peer.Address(idx))
		}
	}

	// wait for all peers to register their new balances
	for _, peer := range n.Peers()[1:] {
		for idx := 0; idx < numRequests; idx++ {
			require.Eventuallyf(t, func() bool {
				balance := tests.Balance(t, peer, peer.Address(idx), ledgerstate.ColorIOTA)
				return balance == uint64(peer.Config().TokensPerRequest)
			}, tests.WaitForDeadline(t), tests.Tick,
				"peer %s did not register its requested funds on address %s", peer, peer.Address(idx).Base58())
		}
	}

	// TODO: why was there a restart before?
}
