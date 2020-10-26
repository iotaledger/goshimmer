package faucet

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMoveAllFunds checks that the faucet moves all funds to its next address on startup.
func TestMoveAllFunds(t *testing.T) {
	prevPoWDiff := framework.ParaPoWDifficulty
	framework.ParaPoWDifficulty = 0
	defer func() {
		framework.ParaPoWDifficulty = prevPoWDiff
	}()
	n, err := f.CreateNetwork("faucet_TestMoveAllFunds", 1, 0, framework.CreateNetworkConfig{Faucet: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for faucet to send initial tx.
	time.Sleep(5 * time.Second)

	facuetPeer := n.Peers()[0]
	addr1 := facuetPeer.Seed.Address(0).String()
	unspentOutputs1, err := facuetPeer.GetUnspentOutputs([]string{addr1})
	require.NoError(t, err)
	assert.Equal(t, 0, len(unspentOutputs1.UnspentOutputs[0].OutputIDs))

	const genesisBalance = int64(1000000000)
	addr2 := facuetPeer.Seed.Address(1).String()
	unspentOutputs2, err := facuetPeer.GetUnspentOutputs([]string{addr2})
	require.NoError(t, err)
	assert.Equal(t, genesisBalance, unspentOutputs2.UnspentOutputs[0].OutputIDs[0].Balances[0].Value)

}
