package faucet

import (
	"context"
	"testing"
	"time"

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
		Activity:    true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peer := n.Peers()[0], n.Peers()[1]
	// use faucet parameters
	var (
		preparedOutputsCount = faucet.Config().PreparedOutputsCount
		tokensPerRequest     = faucet.Config().TokensPerRequest
	)

	// wait for the faucet to prepare all outputs
	require.Eventually(t, func() bool {
		resp, err := faucet.PostAddressUnspentOutputs([]string{faucet.Address(preparedOutputsCount).Base58()})
		require.NoError(t, err)
		return len(resp.UnspentOutputs[0].Outputs) > 0
	}, time.Minute, tests.Tick)

	// check that each of the preparedOutputsCount addresses holds the correct balance
	remainderBalance := uint64(framework.GenesisTokenAmount - preparedOutputsCount*tokensPerRequest)
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))
	for i := 1; i <= preparedOutputsCount; i++ {
		require.EqualValues(t, tokensPerRequest, tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA))
	}

	// consume all the prepared outputs
	for i := 1; i <= preparedOutputsCount; i++ {
		tests.SendFaucetRequest(t, peer, peer.Address(i))
	}

	// wait for the peer to register a balance change
	require.Eventually(t, func() bool {
		return tests.Balance(t, peer, peer.Address(preparedOutputsCount-1), ledgerstate.ColorIOTA) > 0
	}, time.Minute, tests.Tick)

	// check that the remainderBalance is untouched
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))

	// issue one more request to split the remainder balance.
	tests.SendFaucetRequest(t, peer, peer.Address(preparedOutputsCount+1))

	// wait for the faucet to prepare new outputs
	require.Eventually(t, func() bool {
		resp, err := faucet.PostAddressUnspentOutputs([]string{faucet.Address(preparedOutputsCount + preparedOutputsCount).Base58()})
		require.NoError(t, err)
		return len(resp.UnspentOutputs[0].Outputs) > 0
	}, tests.Timeout, tests.Tick)

	// wait for the peer to register the balance change
	require.Eventually(t, func() bool {
		return tests.Balance(t, peer, peer.Address(preparedOutputsCount+1), ledgerstate.ColorIOTA) == uint64(tokensPerRequest)
	}, time.Minute, tests.Tick)

	// ensure that we have spent the balance associated to the request that triggered the split
	require.EqualValues(t, 0, tests.Balance(t, faucet, faucet.Address(preparedOutputsCount+1), ledgerstate.ColorIOTA))

	// check that each of the preparedOutputsCount addresses holds the correct balance
	remainderBalance -= uint64(preparedOutputsCount * tokensPerRequest)
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))

	// we start from preparedOutputsCount + 2 as the address at preparedOutputsCount + 1 is the one already spent
	// that triggered the split
	for i := preparedOutputsCount + 2; i <= preparedOutputsCount+preparedOutputsCount; i++ {
		require.EqualValues(t, tokensPerRequest, tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA))
	}
}
