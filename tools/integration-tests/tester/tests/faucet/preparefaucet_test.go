package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestPrepareFaucet tests that the faucet prepares outputs to be consumed by faucet requests.
func TestPrepareFaucet(t *testing.T) {
	prevPoWDiff := framework.ParaPoWDifficulty
	framework.ParaPoWDifficulty = 0
	defer func() {
		framework.ParaPoWDifficulty = prevPoWDiff
	}()
	n, err := f.CreateNetwork("faucet_testPrepareGenesis", 2, framework.CreateNetworkConfig{Faucet: true, StartSynced: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	faucet := n.Peers()[0]
	preparedOutputsCount := uint64(framework.ParaFaucetPreparedOutputsCount)

	// Tests genesis output is split into 10 outputs. [1,2,...10] and balance,
	const genesisBalance = int64(framework.GenesisTokenAmount)
	var totalSplit int64
	for i := uint64(1); i <= preparedOutputsCount; i++ {
		checkAddressBalance(t, faucet, i, framework.ParaFaucetTokensPerRequest)
		totalSplit += framework.ParaFaucetTokensPerRequest
	}
	// check remaining balances
	remainBalance := genesisBalance - totalSplit
	checkAddressBalance(t, faucet, 0, remainBalance)

	peer := n.Peers()[1]
	var startIndex uint64
	// issue 9 requests to consume the 1st 9 faucet prepared outputs.
	for startIndex = 1; startIndex < preparedOutputsCount; startIndex++ {
		addr := peer.Seed.Address(startIndex).Address()
		tests.SendFaucetRequest(t, peer, addr)
	}
	time.Sleep(5 * time.Second)

	// 1 prepared output is left on the 10th address.
	checkAddressBalance(t, faucet, preparedOutputsCount, framework.ParaFaucetTokensPerRequest)

	// check balance on address(0) is untouched
	checkAddressBalance(t, faucet, 0, remainBalance)

	// issue 2 more request to split the remainder balance.
	for i := uint64(0); i < 2; i++ {
		addr := peer.Seed.Address(startIndex + i).Address()
		tests.SendFaucetRequest(t, peer, addr)
	}
	time.Sleep(5 * time.Second)

	// check split of balance [0] to [11...20]
	checkAddressBalance(t, faucet, 0, genesisBalance-20*framework.ParaFaucetTokensPerRequest)

	for i := uint64(1); i <= preparedOutputsCount; i++ {
		checkAddressBalance(t, faucet, preparedOutputsCount+i, framework.ParaFaucetTokensPerRequest)
	}

}

func checkAddressBalance(t *testing.T, peer *framework.Peer, addrIndex uint64, expectedBalance int64) {
	addr := peer.Seed.Address(addrIndex).Address().Base58()
	outputs, err := peer.PostAddressUnspentOutputs([]string{addr})
	require.NoError(t, err)
	out, err := outputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist := out.Balances().Get(ledgerstate.ColorIOTA)
	assert.Equal(t, true, exist)
	assert.Equal(t, expectedBalance, int64(balanceValue))
}
