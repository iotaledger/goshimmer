package mana

import (
	"math"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManaPersistence(t *testing.T) {
	n, err := f.CreateNetwork("mana_TestPersistence", 1, 0, framework.CreateNetworkConfig{Faucet: true, Mana: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// wait for faucet to move funds and move mana
	time.Sleep(10 * time.Second)

	peers := n.Peers()

	info, err := peers[0].Info()
	require.NoError(t, err)
	manaBefore := info.Mana
	require.Greater(t, manaBefore.Access, 0.0)
	require.Greater(t, manaBefore.Consensus, 0.0)

	// stop all nodes. Expects mana to be saved successfully
	for _, peer := range n.Peers() {
		err = peer.Stop()
		require.NoError(t, err)
	}

	// start all nodes
	for _, peer := range peers {
		err = peer.Start()
		require.NoError(t, err)
	}

	// wait for peers to start
	time.Sleep(5 * time.Second)

	info, err = peers[0].Info()
	require.NoError(t, err)
	manaAfter := info.Mana
	require.Greater(t, manaAfter.Access, 0.0)
	require.Greater(t, manaAfter.Consensus, 0.0)
}

func TestPledgeFilter(t *testing.T) {
	numPeers := 2
	n, err := f.CreateNetwork("mana_TestAPI", 0, 0, framework.CreateNetworkConfig{})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// create peers
	peers := make([]*framework.Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		peer, err := n.CreatePeer(framework.GoShimmerConfig{
			Mana:       true,
			SyncBeacon: true,
		})
		require.NoError(t, err)
		peers[i] = peer
	}

	accessPeer := peers[0]
	accessPeerID := base58.Encode(accessPeer.Identity.ID().Bytes())
	consensusPeer := peers[1]
	consensusPeerID := base58.Encode(consensusPeer.Identity.ID().Bytes())

	// faucet
	faucet, err := n.CreatePeer(framework.GoShimmerConfig{
		Faucet:                            true,
		Mana:                              true,
		ManaAllowedAccessFilterEnabled:    true,
		ManaAllowedConsensusFilterEnabled: true,
		ManaAllowedAccessPledge:           []string{accessPeerID},
		ManaAllowedConsensusPledge:        []string{consensusPeerID},
		SyncBeacon:                        true,
	})

	require.NoError(t, err)
	err = n.WaitForAutopeering(2)
	require.NoError(t, err)

	time.Sleep(10 * time.Second)

	addrBalance := make(map[string]map[ledgerstate.Color]int64)
	faucetAddrStr := faucet.Seed.Address(1).Address().Base58()
	addrBalance[faucetAddrStr] = make(map[ledgerstate.Color]int64)
	addrBalance[accessPeer.Address(0).Address().Base58()] = make(map[ledgerstate.Color]int64)
	addrBalance[consensusPeer.Address(0).Address().Base58()] = make(map[ledgerstate.Color]int64)

	// get faucet balances
	unspentOutputs, err := faucet.GetUnspentOutputs([]string{faucetAddrStr})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", faucet.String())
	addrBalance[faucetAddrStr][ledgerstate.ColorIOTA] = unspentOutputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	// pledge mana to allowed pledge
	fail, _ := tests.SendIotaTransaction(t, faucet, accessPeer, addrBalance, 100, tests.TransactionConfig{
		FromAddressIndex:      1,
		ToAddressIndex:        0,
		AccessManaPledgeID:    accessPeer.Identity.ID(),
		ConsensusManaPledgeID: consensusPeer.Identity.ID(),
	})
	require.False(t, fail)

	// pledge mana to disallowed pledge
	fail, _ = tests.SendIotaTransaction(t, faucet, consensusPeer, addrBalance, 100, tests.TransactionConfig{
		FromAddressIndex:      2,
		ToAddressIndex:        0,
		AccessManaPledgeID:    accessPeer.Identity.ID(),
		ConsensusManaPledgeID: accessPeer.Identity.ID(),
	})
	require.True(t, fail)

	// pledge mana to disallowed pledge
	fail, _ = tests.SendIotaTransaction(t, faucet, consensusPeer, addrBalance, 100, tests.TransactionConfig{
		FromAddressIndex:      2,
		ToAddressIndex:        0,
		AccessManaPledgeID:    consensusPeer.Identity.ID(),
		ConsensusManaPledgeID: consensusPeer.Identity.ID(),
	})
	require.True(t, fail)
}

func TestConsensusManaInThePast(t *testing.T) {
	n, err := f.CreateNetwork("mana_TestConsensusManaInThePast", 2, 1, framework.CreateNetworkConfig{Faucet: true, Mana: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	peers := n.Peers()

	// wait for faucet to move funds and move mana
	time.Sleep(5 * time.Second)

	// send funds
	tests.SendTransactionFromFaucet(t, peers[:], 100)
	time.Sleep(10 * time.Second)
	timeInPast := time.Now()

	// so that we update mana vectors
	_, _ = peers[0].GetAllMana()

	// move more funds to update base mana vectors
	tests.SendTransactionFromFaucet(t, peers[:], 50)
	time.Sleep(10 * time.Second)

	res1, err := peers[0].GetPastConsensusManaVector(timeInPast.Unix())
	require.NoError(t, err)
	for _, c := range res1.Consensus {
		require.Greater(t, c.Mana, 0.0)
	}

	// TODO: do a more useful test. e.g compare mana now vs mana in timeInPast
}

func TestApis(t *testing.T) {
	prevParaManaOnEveryNode := framework.ParaManaOnEveryNode
	framework.ParaManaOnEveryNode = true
	defer func() {
		framework.ParaManaOnEveryNode = prevParaManaOnEveryNode
	}()
	n, err := f.CreateNetwork("mana_TestAPI", 3, 2, framework.CreateNetworkConfig{Faucet: true, Mana: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	peers := n.Peers()

	// Test /mana
	resp, err := peers[0].GoShimmerAPI.GetManaFullNodeID(base58.Encode(peers[0].ID().Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp.NodeID)
	assert.Greater(t, resp.Access, 0.0)
	assert.Greater(t, resp.Consensus, 0.0)

	// Test /mana/all
	resp2, err := peers[0].GoShimmerAPI.GetAllMana()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp2.Access))
	assert.Greater(t, resp2.Access[0].Mana, 0.0)

	// Test /mana/access/nhighest and /mana/consensus/nhighest

	// send funds to node 1
	tests.SendTransactionFromFaucet(t, peers[:2], 1337)
	time.Sleep(2 * time.Second)
	resp3, err := peers[0].GoShimmerAPI.GetNHighestAccessMana(len(peers))
	assert.NoError(t, err)
	resp4, err := peers[0].GoShimmerAPI.GetNHighestConsensusMana(len(peers))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(resp3.Nodes))
	assert.Equal(t, 2, len(resp4.Nodes))
	for i := 0; i < 2; i++ {
		assert.Equal(t, base58.Encode(peers[i].ID().Bytes()), resp3.Nodes[i].NodeID)
		assert.Equal(t, base58.Encode(peers[i].ID().Bytes()), resp4.Nodes[i].NodeID)
	}

	// Test /mana/percentile
	resp5, err := peers[0].GoShimmerAPI.GetManaPercentile(base58.Encode(peers[0].ID().Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp5.NodeID)
	assert.Equal(t, 50.0, math.Round(resp5.Access*100)/100)
	assert.Equal(t, 50.0, math.Round(resp5.Consensus*100)/100)
}
