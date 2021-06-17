package faucet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestFaucetPrepare tests that the faucet prepares outputs to be consumed by faucet requests.
func TestFaucetPrepare(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 2, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peer := n.Peers()[0], n.Peers()[1]
	// use faucet parameters
	var (
		preparedOutputsCounts = faucet.Config().PreparedOutputsCounts
		tokensPerRequest      = faucet.Config().TokensPerRequest
	)

	// wait for the faucet to prepare all outputs
	require.Eventually(t, func() bool {
		resp, err := faucet.PostAddressUnspentOutputs([]string{faucet.Address(preparedOutputsCounts).Base58()})
		require.NoError(t, err)
		return len(resp.UnspentOutputs[0].Outputs) > 0
	}, tests.WaitForDeadline(t), tests.Tick)

	// check that each of the preparedOutputsCounts addresses holds the correct balance
	remainderBalance := uint64(framework.GenesisTokenAmount - preparedOutputsCounts*tokensPerRequest)
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))
	for i := 1; i <= preparedOutputsCounts; i++ {
		require.EqualValues(t, tokensPerRequest, tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA))
	}

	// consume all but one of the prepared outputs
	for i := 1; i < preparedOutputsCounts; i++ {
		tests.SendFaucetRequest(t, peer, peer.Address(i))
	}

	// wait for the peer to register a balance change
	require.Eventually(t, func() bool {
		return tests.Balance(t, peer, peer.Address(preparedOutputsCounts-1), ledgerstate.ColorIOTA) > 0
	}, tests.WaitForDeadline(t), tests.Tick)

	// one prepared output is left on the last address.
	require.EqualValues(t, tokensPerRequest, tests.Balance(t, faucet, faucet.Address(preparedOutputsCounts), ledgerstate.ColorIOTA))

	// check that the remainderBalance is untouched
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))

	// issue two more request to split the remainder balance.
	tests.SendFaucetRequest(t, peer, peer.Address(preparedOutputsCounts))
	tests.SendFaucetRequest(t, peer, peer.Address(preparedOutputsCounts+1))

	// wait for the faucet to prepare new outputs
	require.Eventually(t, func() bool {
		resp, err := faucet.PostAddressUnspentOutputs([]string{faucet.Address(preparedOutputsCounts + preparedOutputsCounts).Base58()})
		require.NoError(t, err)
		return len(resp.UnspentOutputs[0].Outputs) > 0
	}, tests.WaitForDeadline(t), tests.Tick)

	// check that each of the preparedOutputsCounts addresses holds the correct balance
	remainderBalance -= uint64(preparedOutputsCounts * tokensPerRequest)
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))
	for i := preparedOutputsCounts + 1; i <= preparedOutputsCounts+preparedOutputsCounts; i++ {
		require.EqualValues(t, tokensPerRequest, tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA))
	}
}
