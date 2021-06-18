package mana

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManaPersistence(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 2, framework.CreateNetworkConfig{
		Faucet:      true,
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peer := n.Peers()[0], n.Peers()[1]

	info, err := faucet.Info()
	require.NoError(t, err)
	manaBefore := info.Mana
	require.Greater(t, manaBefore.Access, 0.0)
	require.Greater(t, manaBefore.Consensus, 0.0)

	tests.SendFaucetRequest(t, peer, peer.Address(0))

	// restart the peer
	err = peer.Stop()
	require.NoError(t, err)
	err = peer.Start()
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	info, err := peer.Info()
	require.NoError(t, err)
	manaAfter := info.Mana
	require.Greater(t, manaAfter.Access, 10.0)
	require.Greater(t, manaAfter.Consensus, 10.0)
}

func TestManaPledgeFilter(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 2, framework.CreateNetworkConfig{
		Faucet:      true, // TODO: do we need the faucet here?
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	peers := n.Peers()

	accessPeer := peers[0]
	accessPeerID := base58.Encode(accessPeer.Identity.ID().Bytes())
	consensusPeer := peers[1]
	consensusPeerID := base58.Encode(consensusPeer.Identity.ID().Bytes())

	faucetConfig := framework.PeerConfig
	faucetConfig.MessageLayer.StartSynced = true // TODO: how can we make it sync?
	faucetConfig.Faucet.Enabled = true
	faucetConfig.Mana = config.Mana{
		Enabled:                       true,
		AllowedAccessPledge:           []string{accessPeerID},
		AllowedAccessFilterEnabled:    true,
		AllowedConsensusPledge:        []string{consensusPeerID},
		AllowedConsensusFilterEnabled: true,
	}
	faucet, err := n.CreatePeer(ctx, faucetConfig)
	require.NoError(t, err)

	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// wait for the faucet to prepare all outputs
	require.Eventually(t, func() bool {
		resp, err := faucet.PostAddressUnspentOutputs([]string{faucet.Address(1).Base58()})
		require.NoError(t, err)
		return len(resp.UnspentOutputs[0].Outputs) > 0
	}, tests.WaitForDeadline(t), tests.Tick)

	addrBalance := make(map[string]map[ledgerstate.Color]int64)
	faucetAddrStr := faucet.Address(1).Base58()
	addrBalance[faucetAddrStr] = make(map[ledgerstate.Color]int64)
	addrBalance[accessPeer.Address(0).Base58()] = make(map[ledgerstate.Color]int64)
	addrBalance[consensusPeer.Address(0).Base58()] = make(map[ledgerstate.Color]int64)

	// get faucet balances
	unspentOutputs, err := faucet.PostAddressUnspentOutputs([]string{faucetAddrStr})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", faucet.String())
	out, err := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist := out.Balances().Get(ledgerstate.ColorIOTA)
	require.Equal(t, true, exist)
	addrBalance[faucetAddrStr][ledgerstate.ColorIOTA] = int64(balanceValue)

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

func TestManaApis(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		Faucet:      true, // TODO: do we need the faucet here?
		StartSynced: true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	err = tests.AwaitSync(t, n.Peers(), 20*time.Second)
	require.NoError(t, err)

	emptyNodeID := identity.ID{}

	peers := n.Peers()
	for _, p := range peers {
		fmt.Printf("peer id: %s, short id: %s\n", base58.Encode(p.ID().Bytes()), p.ID().String())
	}
	faucet := peers[0]

	// Test /mana
	// consensus mana was pledged to empty nodeID by faucet
	resp, err := peers[0].GoShimmerAPI.GetManaFullNodeID(base58.Encode(emptyNodeID.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(emptyNodeID.Bytes()), resp.NodeID)
	assert.Equal(t, 1.0, resp.Access)
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
	peer1ID := base58.Encode(peers[1].ID().Bytes())
	_, err = peers[0].SendFaucetRequest(peers[1].Seed.Address(0).Address().Base58(), faucet.Config().Faucet.PowDifficulty, peer1ID, peer1ID)
	require.NoError(t, err)
	time.Sleep(30 * time.Second)
	// send funds to node 2
	peer2ID := base58.Encode(peers[2].ID().Bytes())
	_, err = peers[0].SendFaucetRequest(peers[2].Seed.Address(0).Address().Base58(), faucet.Config().Faucet.PowDifficulty, peer2ID, peer2ID)
	require.NoError(t, err)
	time.Sleep(10 * time.Second)

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
	unspentOutputs, err := peers[1].PostAddressUnspentOutputs([]string{peers[1].Seed.Address(0).Address().Base58()})
	require.NoError(t, err)
	outputID := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.OutputID.Base58
	resp8, err := peers[1].GetPending(outputID)
	require.NoError(t, err)
	assert.Equal(t, outputID, resp8.OutputID)
	fmt.Println("pending mana: ", resp8.Mana)
	assert.Greater(t, resp8.Mana, 0.0)

	// Test /mana/allowedManaPledge
	pledgeList, err := peers[0].GoShimmerAPI.GetAllowedManaPledgeNodeIDs()
	require.NoError(t, err, "Error occurred while testing allowed mana pledge api")
	assert.Equal(t, false, pledgeList.Access.IsFilterEnabled, "/mana/allowedManaPledge: FilterEnabled field for access mana is not as expected")
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), pledgeList.Access.Allowed[0], "/mana/allowedManaPledge: Allowed node id to pledge access mana is not as expected")
	assert.Equal(t, false, pledgeList.Consensus.IsFilterEnabled, "/mana/allowedManaPledge: FilterEnabled field for consensus mana is not as expected")
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), pledgeList.Consensus.Allowed[0], "/mana/allowedManaPledge: Allowed node id to pledge consensus mana is not as expected")
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
