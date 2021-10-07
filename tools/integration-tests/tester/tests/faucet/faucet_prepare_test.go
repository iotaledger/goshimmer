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
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Activity:    true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peer := n.Peers()[0], n.Peers()[1]
	// use faucet parameters
	var (
		supplyOutputsCount      = faucet.Config().SupplyOutputsCount
		splittingMultiplayer    = faucet.Config().SplittingMultiplier
		tokensPerRequest        = faucet.Config().TokensPerRequest
		fundingOutputsAddrStart = tests.FaucetFundingOutputsAddrStart
		lastFundingOutputAddr   = supplyOutputsCount*splittingMultiplayer + fundingOutputsAddrStart - 1
	)

	// wait for the faucet to split the supply tx and prepare all outputs
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet)

	// check that each of the supplyOutputsCount addresses holds the correct balance
	remainderBalance := uint64(framework.GenesisTokenAmount - supplyOutputsCount*splittingMultiplayer*tokensPerRequest)
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))
	for i := fundingOutputsAddrStart; i <= lastFundingOutputAddr; i++ {
		require.EqualValues(t, uint64(tokensPerRequest), tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA))
	}

	// consume all but one of the prepared outputs
	for i := 1; i < supplyOutputsCount*splittingMultiplayer; i++ {
		tests.SendFaucetRequest(t, peer, peer.Address(i))
	}

	// wait for the peer to register a balance change
	require.Eventually(t, func() bool {
		return tests.Balance(t, peer, peer.Address(supplyOutputsCount*splittingMultiplayer-1), ledgerstate.ColorIOTA) > 0
	}, tests.Timeout, tests.Tick)

	// one prepared output is left from the first prepared batch, index is not known because outputs are not sorted by the index.
	var balanceLeft uint64 = 0
	for i := fundingOutputsAddrStart; i <= lastFundingOutputAddr; i++ {
		balanceLeft += tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA)
	}
	require.EqualValues(t, uint64(tokensPerRequest), balanceLeft)

	// check that more funds preparation has been triggered
	// wait for the faucet to finish preparing new outputs
	require.Eventually(t, func() bool {
		resp, err := faucet.PostAddressUnspentOutputs([]string{faucet.Address(lastFundingOutputAddr + splittingMultiplayer*supplyOutputsCount - 1).Base58()})
		require.NoError(t, err)
		return len(resp.UnspentOutputs[0].Outputs) > 0
	}, tests.Timeout, tests.Tick)

	// check that each of the supplyOutputsCount addresses holds the correct balance
	for i := lastFundingOutputAddr + 1; i <= lastFundingOutputAddr+splittingMultiplayer*supplyOutputsCount; i++ {
		require.EqualValues(t, uint64(tokensPerRequest), tests.Balance(t, faucet, faucet.Address(i), ledgerstate.ColorIOTA))
	}

	// check that remainder has correct balance
	remainderBalance -= uint64(supplyOutputsCount * tokensPerRequest * splittingMultiplayer)
	require.EqualValues(t, remainderBalance, tests.Balance(t, faucet, faucet.Address(0), ledgerstate.ColorIOTA))
}
