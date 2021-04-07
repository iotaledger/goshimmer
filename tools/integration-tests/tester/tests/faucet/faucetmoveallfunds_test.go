package faucet

import (
	"testing"
	"time"

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
		Faucet:     true,
		Mana:       true,
		SyncBeacon: true,
	})
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	// Tests genesis output is split into 10 outputs. [1,2,...10] and balance,
	const genesisBalance = int64(1000000000000000)
	var totalSplit int64
	var i uint64
	for i = 1; i <= 10; i++ {
		addr := faucet.Seed.Address(i).Address().Base58()
		outputs, err := faucet.GetUnspentOutputs([]string{addr})
		require.NoError(t, err)
		assert.Equal(t, framework.ParaFaucetTokensPerRequest, outputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)
		totalSplit += framework.ParaFaucetTokensPerRequest
	}
	balance := genesisBalance - totalSplit
	faucetAddr := faucet.Seed.Address(0).Address().Base58()
	outputs, err := faucet.GetUnspentOutputs([]string{faucetAddr})
	require.NoError(t, err)
	assert.Equal(t, balance, outputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)

	// add 1 node to the network
	peer, err := n.CreatePeer(framework.GoShimmerConfig{
		Mana:       true,
		SyncBeacon: true,
	})
	require.NoError(t, err)

	err = n.WaitForAutopeering(1)
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	// issue 9 requests to consume the 1st 9 faucet prepared outputs.
	for i = 0; i < 9; i++ {
		addr := peer.Address(i).Address()
		tests.SendFaucetRequest(t, peer, addr)
	}
	time.Sleep(5 * time.Second)

	// 1 prepared output is left on the 10th address.
	lastPreparedOutputAddress := faucet.Seed.Address(10).Address().Base58()
	lastPreparedOutput, err := faucet.GetUnspentOutputs([]string{lastPreparedOutputAddress})
	require.NoError(t, err)
	assert.Equal(t, framework.ParaFaucetTokensPerRequest, lastPreparedOutput.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)

	// check balance is untouched
	balanceOutputAddress := faucet.Seed.Address(0).Address().Base58()
	balanceOutput, err := faucet.GetUnspentOutputs([]string{balanceOutputAddress})
	require.NoError(t, err)
	assert.Equal(t, genesisBalance-(10*framework.ParaFaucetTokensPerRequest), balanceOutput.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)

	// issue 2 more request to split the remainder balance.
	addr := peer.Seed.Address(10).Address()
	tests.SendFaucetRequest(t, peer, addr)
	addr = peer.Seed.Address(11).Address()
	tests.SendFaucetRequest(t, peer, addr)
	time.Sleep(2 * time.Second)

	// check split of balance [0] to [11...20]
	_addr := faucet.Seed.Address(0).Address().Base58()
	outputs, err = faucet.GetUnspentOutputs([]string{_addr})
	require.NoError(t, err)
	assert.Equal(t, 1, len(outputs.UnspentOutputs[0].OutputIDs)) // 1 output is unspent.
	assert.Equal(t, genesisBalance-20*framework.ParaFaucetTokensPerRequest, outputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)

	for i := 11; i < 21; i++ {
		_addr := faucet.Seed.Address(uint64(i)).Address().Base58()
		outputs, err = faucet.GetUnspentOutputs([]string{_addr})
		require.NoError(t, err)
		assert.Equal(t, framework.ParaFaucetTokensPerRequest, outputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)
	}

}
