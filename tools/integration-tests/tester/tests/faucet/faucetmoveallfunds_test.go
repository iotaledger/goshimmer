package faucet

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestPrepareFaucet tests that the faucet prepares outputs to be consumed by faucet requests.
func TestPrepareFaucet(t *testing.T) {
	prevPoWDiff := framework.ParaPoWDifficulty
	prevFaucetPreparedOutputsCount := framework.ParaFaucetPreparedOutputsCount
	framework.ParaPoWDifficulty = 0
	framework.ParaFaucetPreparedOutputsCount = 10
	defer func() {
		framework.ParaPoWDifficulty = prevPoWDiff
		framework.ParaFaucetPreparedOutputsCount = prevFaucetPreparedOutputsCount
	}()
	n, err := f.CreateNetwork("faucet_testPrepareGenesis", 0, 0, framework.CreateNetworkConfig{Faucet: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	faucet, err := n.CreatePeer(framework.GoShimmerConfig{
		Seed:           "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E",
		Faucet:         true,
		Mana:           true,
		ActivityPlugin: true,
		StartSynced:    true,
	})
	require.NoError(t, err)
	time.Sleep(15 * time.Second)

	// Tests genesis output is split into 10 outputs. [1,2,...10] and balance,
	const genesisBalance = int64(1000000000000000)
	var totalSplit int64
	var i uint64
	for i = 1; i <= 10; i++ {
		addr := faucet.Seed.Address(i).Address().Base58()
		outputs, err := faucet.PostAddressUnspentOutputs([]string{addr})
		require.NoError(t, err)
		out, err := outputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
		require.NoError(t, err)
		balance, exist := out.Balances().Get(ledgerstate.ColorIOTA)
		assert.Equal(t, true, exist)
		assert.Equal(t, framework.ParaFaucetTokensPerRequest, int64(balance))
		totalSplit += framework.ParaFaucetTokensPerRequest
	}
	balance := genesisBalance - totalSplit
	faucetAddr := faucet.Seed.Address(0).Address().Base58()
	outputs, err := faucet.PostAddressUnspentOutputs([]string{faucetAddr})
	require.NoError(t, err)
	out, err := outputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist := out.Balances().Get(ledgerstate.ColorIOTA)
	assert.Equal(t, true, exist)
	assert.Equal(t, balance, int64(balanceValue))

	// add 1 node to the network
	peer, err := n.CreatePeerWithMana(framework.GoShimmerConfig{
		Mana:           true,
		ActivityPlugin: true,
	})
	require.NoError(t, err)

	err = n.WaitForAutopeering(1)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	// issue 9 requests to consume the 1st 9 faucet prepared outputs.
	for i = 1; i < 9; i++ {
		addr := peer.Address(i).Address()
		tests.SendFaucetRequest(t, peer, addr)
	}
	time.Sleep(5 * time.Second)

	// 1 prepared output is left on the 10th address.
	lastPreparedOutputAddress := faucet.Seed.Address(10).Address().Base58()
	lastPreparedOutput, err := faucet.PostAddressUnspentOutputs([]string{lastPreparedOutputAddress})
	require.NoError(t, err)
	out, err = lastPreparedOutput.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist = out.Balances().Get(ledgerstate.ColorIOTA)
	assert.Equal(t, true, exist)
	assert.Equal(t, framework.ParaFaucetTokensPerRequest, int64(balanceValue))

	// check balance is untouched
	balanceOutputAddress := faucet.Seed.Address(0).Address().Base58()
	balanceOutput, err := faucet.PostAddressUnspentOutputs([]string{balanceOutputAddress})
	require.NoError(t, err)
	out, err = balanceOutput.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist = out.Balances().Get(ledgerstate.ColorIOTA)
	assert.Equal(t, true, exist)
	assert.Equal(t, genesisBalance-(10*framework.ParaFaucetTokensPerRequest), int64(balanceValue))

	// issue 2 more request to split the remainder balance.
	addr := peer.Seed.Address(10).Address()
	tests.SendFaucetRequest(t, peer, addr)
	addr = peer.Seed.Address(11).Address()
	tests.SendFaucetRequest(t, peer, addr)
	time.Sleep(2 * time.Second)

	// check split of balance [0] to [11...20]
	_addr := faucet.Seed.Address(0).Address().Base58()
	outputs, err = faucet.PostAddressUnspentOutputs([]string{_addr})
	require.NoError(t, err)
	assert.Equal(t, 1, len(outputs.UnspentOutputs[0].Outputs)) // 1 output is unspent.
	out, err = outputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist = out.Balances().Get(ledgerstate.ColorIOTA)
	assert.Equal(t, true, exist)
	assert.Equal(t, genesisBalance-20*framework.ParaFaucetTokensPerRequest, int64(balanceValue))

	for i := 11; i < 21; i++ {
		_addr := faucet.Seed.Address(uint64(i)).Address().Base58()
		outputs, err = faucet.PostAddressUnspentOutputs([]string{_addr})
		require.NoError(t, err)
		out, err = outputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
		require.NoError(t, err)
		balanceValue, exist = out.Balances().Get(ledgerstate.ColorIOTA)
		assert.Equal(t, true, exist)
		assert.Equal(t, framework.ParaFaucetTokensPerRequest, int64(balanceValue))
	}

}
