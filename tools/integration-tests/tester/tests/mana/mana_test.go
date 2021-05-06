package mana

import (
	"fmt"
	"testing"
	"time"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/identity"
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
		Seed:                              "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E",
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

func TestApis(t *testing.T) {
	prevParaManaOnEveryNode := framework.ParaManaOnEveryNode
	framework.ParaManaOnEveryNode = true
	defer func() {
		framework.ParaManaOnEveryNode = prevParaManaOnEveryNode
	}()
	n, err := f.CreateNetwork("mana_TestAPI", 4, 3, framework.CreateNetworkConfig{Faucet: true, Mana: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	emptyNodeID := identity.ID{}

	peers := n.Peers()
	for _, p := range peers {
		fmt.Printf("peer id: %s, short id: %s\n", base58.Encode(p.ID().Bytes()), p.ID().String())
	}

	// Test /mana
	// consensus mana was pledged to empty nodeID by faucet
	resp, err := peers[0].GoShimmerAPI.GetManaFullNodeID(base58.Encode(emptyNodeID.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(emptyNodeID.Bytes()), resp.NodeID)
	assert.Equal(t, 0.0, resp.Access)
	assert.Greater(t, resp.Consensus, 0.0)

	resp, err = peers[0].GoShimmerAPI.GetManaFullNodeID(base58.Encode(peers[0].ID().Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp.NodeID)
	assert.Greater(t, resp.Access, 0.0)
	assert.Greater(t, resp.Consensus, 0.0)

	// Test /mana/all
	resp2, err := peers[0].GoShimmerAPI.GetAllMana()
	require.NoError(t, err)
	assert.Equal(t, 2, len(resp2.Access))
	assert.Greater(t, resp2.Access[0].Mana, 0.0)

	// Test /mana/access/nhighest and /mana/consensus/nhighest
	// send funds to node 1
	_, err = peers[1].SendFaucetRequest(peers[1].Seed.Address(0).Address().Base58())
	require.NoError(t, err)
	time.Sleep(10 * time.Second)
	// send funds to node 2
	_, err = peers[2].SendFaucetRequest(peers[2].Seed.Address(0).Address().Base58())
	require.NoError(t, err)
	time.Sleep(20 * time.Second)

	require.NoError(t, err)
	allManaResp, err := peers[0].GoShimmerAPI.GetAllMana()
	require.NoError(t, err)
	fmt.Println("all mana")
	for _, m := range allManaResp.Access {
		fmt.Println("nodeid: ", m.NodeID, " mana: ", m.Mana)
	}
	resp3, err := peers[0].GoShimmerAPI.GetNHighestAccessMana(len(peers) + 2)
	t.Log("resp3", resp3)
	require.NoError(t, err)
	resp3.Nodes = stripGenesisNodeID(resp3.Nodes)
	resp4, err := peers[0].GoShimmerAPI.GetNHighestConsensusMana(len(peers) + 2)
	t.Log("resp", resp4)
	require.NoError(t, err)
	resp4.Nodes = stripGenesisNodeID(resp4.Nodes)
	require.Equal(t, 3, len(resp3.Nodes))
	for i := 0; i < 3; i++ {
		assert.Equal(t, base58.Encode(peers[i].ID().Bytes()), resp3.Nodes[i].NodeID)
	}

	require.Equal(t, 4, len(resp4.Nodes))
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp4.Nodes[0].NodeID)
	assert.Equal(t, base58.Encode(emptyNodeID.Bytes()), resp4.Nodes[1].NodeID)
	assert.True(t, base58.Encode(peers[1].ID().Bytes()) == resp4.Nodes[2].NodeID || base58.Encode(peers[1].ID().Bytes()) == resp4.Nodes[3].NodeID)
	assert.True(t, base58.Encode(peers[2].ID().Bytes()) == resp4.Nodes[2].NodeID || base58.Encode(peers[2].ID().Bytes()) == resp4.Nodes[3].NodeID)

	// Test /mana/percentile
	resp5, err := peers[0].GoShimmerAPI.GetManaPercentile(base58.Encode(peers[0].ID().Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp5.NodeID)
	assert.InDelta(t, 75.0, resp5.Access, 0.01)

	resp5, err = peers[0].GoShimmerAPI.GetManaPercentile(base58.Encode(emptyNodeID.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(emptyNodeID.Bytes()), resp5.NodeID)
	assert.InDelta(t, 60., resp5.Consensus, 0.01)

	// Test /mana/online/access
	resp6, err := peers[0].GoShimmerAPI.GetOnlineAccessMana()
	require.NoError(t, err)
	resp7, err := peers[0].GoShimmerAPI.GetOnlineConsensusMana()
	require.NoError(t, err)
	require.Equal(t, 3, len(resp6.Online))
	// emptyNodeID cannot be online!
	require.Equal(t, 3, len(resp7.Online))
	fmt.Println("online nodes access mana")
	for _, r := range resp6.Online {
		fmt.Println("node - ", r.ShortID, " -- mana: ", r.Mana)
	}
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp6.Online[0].ID)
	assert.Equal(t, base58.Encode(peers[1].ID().Bytes()), resp6.Online[1].ID)
	assert.Equal(t, base58.Encode(peers[2].ID().Bytes()), resp6.Online[2].ID)

	fmt.Println("online nodes consensus mana")
	for _, r := range resp7.Online {
		fmt.Println("node - ", r.ShortID, " -- mana: ", r.Mana)
	}
	// emptyNodeID cannot be online!
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp7.Online[0].ID)
	assert.True(t, base58.Encode(peers[1].ID().Bytes()) == resp7.Online[1].ID ||
		base58.Encode(peers[1].ID().Bytes()) == resp7.Online[2].ID)

	// Test /mana/pending
	unspentOutputs, err := peers[1].GetUnspentOutputs([]string{peers[1].Seed.Address(0).Address().Base58()})
	require.NoError(t, err)
	outputID := unspentOutputs.UnspentOutputs[0].OutputIDs[0].ID
	resp8, err := peers[1].GetPending(outputID)
	require.NoError(t, err)
	assert.Equal(t, outputID, resp8.OutputID)
	fmt.Println("pending mana: ", resp8.Mana)
	assert.Greater(t, resp8.Mana, 0.0)
}

func stripGenesisNodeID(input []manaPkg.NodeStr) (output []manaPkg.NodeStr) {
	peerMaster := "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"
	// faucet := "FZ6xmPZXRs2M8z9m9ETTQok4PCga4X8FRHwQE6uYm4rV"
	for _, id := range input {
		if id.NodeID == peerMaster {
			continue
		}
		output = append(output, id)
	}
	return
}
