package faucet

import (
	"context"
	"log"
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
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
	snapshotOptions := tests.EqualSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)
	n, err := f.CreateNetwork(ctx, t.Name(), numPeers, framework.CreateNetworkConfig{
		StartSynced: false,
		Faucet:      true,
		Activity:    true,
		Snapshot:    snapshotOptions,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)
	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	faucet, nonFaucetPeers := n.Peers()[0], n.Peers()[1:]

	// check consensus mana: all nodes should have equal mana
	require.Eventually(t, func() bool {
		return tests.Mana(t, faucet).Consensus > 0
	}, tests.Timeout, tests.Tick)
	require.EqualValues(t, snapshotInfo.GenesisTokenAmount, tests.Mana(t, faucet).Consensus)

	for i, peer := range nonFaucetPeers {
		if snapshotInfo.PeersAmountsPledged[i] > 0 {
			require.Eventually(t, func() bool {
				return tests.Mana(t, peer).Consensus > 0
			}, tests.Timeout, tests.Tick)
		}
		require.EqualValues(t, snapshotInfo.PeersAmountsPledged[i], tests.Mana(t, peer).Consensus)
	}

	// each non-faucet peer issues numRequests requests
	for _, peer := range nonFaucetPeers {
		for idx := 0; idx < numRequests; idx++ {
			tests.SendFaucetRequest(t, peer, peer.Address(idx))
		}
	}

	// wait for all peers to register their new balances
	for _, peer := range nonFaucetPeers {
		for idx := 0; idx < numRequests; idx++ {
			require.Eventuallyf(t, func() bool {
				balance := tests.Balance(t, peer, peer.Address(idx), devnetvm.ColorIOTA)
				return balance == uint64(faucet.Config().TokensPerRequest)
			}, tests.Timeout, tests.Tick,
				"peer %s did not register its requested funds on address %s", peer, peer.Address(idx).Base58())
		}
	}
}
