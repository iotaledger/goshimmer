package faucet

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrepareGenesis tests that the faucet splits genesis funds to CfgFaucetPreparedOutputsCount outputs.
func TestPrepareGenesis(t *testing.T) {
	prevPoWDiff := framework.ParaPoWDifficulty
	framework.ParaPoWDifficulty = 0
	defer func() {
		framework.ParaPoWDifficulty = prevPoWDiff
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
	time.Sleep(10 * time.Second)

	const genesisBalance = int64(1000000000)
	var totalSplit int64
	var i uint64
	for i = 1; i <= 10; i++ {
		addr := faucet.Seed.Address(i).String()
		outputs, err := faucet.GetUnspentOutputs([]string{addr})
		require.NoError(t, err)
		assert.Equal(t, framework.ParaFaucetTokensPerRequest, outputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)
		totalSplit += framework.ParaFaucetTokensPerRequest
	}
	balance := genesisBalance - totalSplit
	addr := faucet.Seed.Address(i).String()
	outputs, err := faucet.GetUnspentOutputs([]string{addr})
	require.NoError(t, err)
	assert.Equal(t, balance, outputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)
}
