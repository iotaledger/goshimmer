package mana

import (
	"context"
	"log"
	"math"
	"testing"

	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	faucetplugin "github.com/iotaledger/goshimmer/plugins/faucet"
	webapiledgerstate "github.com/iotaledger/goshimmer/plugins/webapi/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

var (
	// minAccessMana is minimal amout of mana required to access the network
	minAccessMana    = tangle.MinMana
	minConsensusMana = 0.0

	emptyNodeID = identity.ID{}
)

func TestManaPersistence(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotInfo := tests.EqualSnapshotDetails
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		Faucet:      true,
		StartSynced: true,
		Activity:    true,
		PeerMaster:  true,
		Snapshots:   []framework.SnapshotInfo{snapshotInfo},
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peer := n.Peers()[0], n.Peers()[1]

	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())
	tests.SendFaucetRequest(t, peer, peer.Address(0))

	log.Println("Waiting for peer to get access mana...")
	require.Eventually(t, func() bool {
		return tests.Mana(t, peer).Access > minAccessMana
	}, tests.Timeout, tests.Tick)
	log.Println("Waiting for peer to get consensus mana...")
	require.Eventually(t, func() bool {
		return tests.Mana(t, peer).Consensus > 0
	}, tests.Timeout, tests.Tick)
	log.Println("Waiting for peer to get mana... done")

	// restart the peer
	require.NoError(t, peer.Restart(ctx))

	require.Greater(t, tests.Mana(t, peer).Access, minAccessMana)
	require.Greater(t, tests.Mana(t, peer).Consensus, minConsensusMana)
}

func TestManaPledgeFilter(t *testing.T) {
	const (
		numPeers         = 3
		tokensPerRequest = 100
	)
	snapshotInfo := tests.EqualSnapshotDetails
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), numPeers, framework.CreateNetworkConfig{
		StartSynced: true,
		Activity:    true,
		PeerMaster:  false,
		Snapshots:   []framework.SnapshotInfo{snapshotInfo},
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	peers := n.Peers()

	accessPeer := peers[0]
	accessPeerID := fullID(accessPeer.ID())
	consensusPeer := peers[1]
	consensusPeerID := fullID(consensusPeer.ID())
	seedBytes, err := base58.Decode(snapshotInfo.MasterSeed)
	require.NoError(t, err)

	faucetConfig := framework.PeerConfig()
	faucetConfig.MessageLayer.StartSynced = true
	faucetConfig.Faucet.Enabled = true
	faucetConfig.Mana.Enabled = true
	faucetConfig.Mana.AllowedAccessPledge = []string{accessPeerID}
	faucetConfig.Mana.AllowedAccessFilterEnabled = true
	faucetConfig.Mana.AllowedConsensusPledge = []string{consensusPeerID}
	faucetConfig.Mana.AllowedConsensusFilterEnabled = true
	faucetConfig.Activity.Enabled = true
	faucetConfig.Seed = seedBytes
	faucetConfig.MessageLayer.Snapshot.File = snapshotInfo.FilePath

	faucet, err := n.CreatePeer(ctx, faucetConfig)
	require.NoError(t, err)

	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// wait for the faucet to prepare all outputs
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	var faucetStartAddress uint64 = faucetplugin.MaxFaucetOutputsCount + 1
	// pledge mana to allowed peers
	_, err = tests.SendTransaction(t, faucet, accessPeer, ledgerstate.ColorIOTA, tokensPerRequest, tests.TransactionConfig{
		FromAddressIndex:      faucetStartAddress,
		ToAddressIndex:        0,
		AccessManaPledgeID:    accessPeer.Identity.ID(),
		ConsensusManaPledgeID: consensusPeer.Identity.ID(),
	})
	require.NoError(t, err)

	// pledge consensus mana to forbidden peer
	_, err = tests.SendTransaction(t, faucet, accessPeer, ledgerstate.ColorIOTA, tokensPerRequest, tests.TransactionConfig{
		FromAddressIndex:      faucetStartAddress,
		ToAddressIndex:        0,
		AccessManaPledgeID:    accessPeer.Identity.ID(),
		ConsensusManaPledgeID: accessPeer.Identity.ID(),
	})
	require.Contains(t, err.Error(), webapiledgerstate.ErrNotAllowedToPledgeManaToNode.Error())

	// pledge access mana to forbidden peer
	_, err = tests.SendTransaction(t, faucet, accessPeer, ledgerstate.ColorIOTA, tokensPerRequest, tests.TransactionConfig{
		FromAddressIndex:      faucetStartAddress,
		ToAddressIndex:        0,
		AccessManaPledgeID:    consensusPeer.Identity.ID(),
		ConsensusManaPledgeID: consensusPeer.Identity.ID(),
	})
	require.ErrorIs(t, err, client.ErrBadRequest)
	require.Contains(t, err.Error(), webapiledgerstate.ErrNotAllowedToPledgeManaToNode.Error())
}

func TestManaApis(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotInfo := tests.EqualSnapshotDetails
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Autopeering: true, // we need to discover online peers
		Activity:    true, // we need to issue regular activity messages
		PeerMaster:  true,
		Snapshots:   []framework.SnapshotInfo{snapshotInfo},
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	peers := n.Peers()
	faucet := peers[0]

	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	log.Println("Request mana from faucet...")
	// waiting for the faucet to have access mana
	require.Eventually(t, func() bool {
		return tests.Mana(t, faucet).Access > minAccessMana
	}, tests.Timeout, tests.Tick)

	// request mana for peer #1; do this twice to assure that peer #1 gets more mana than peer #2
	tests.SendFaucetRequest(t, peers[1], peers[1].Address(0))
	tests.SendFaucetRequest(t, peers[1], peers[1].Address(1))

	require.Eventually(t, func() bool {
		return tests.Mana(t, peers[1]).Access > minAccessMana
	}, tests.Timeout, tests.Tick)

	// request mana for peer #2
	tests.SendFaucetRequest(t, peers[2], peers[2].Address(0))

	require.Eventually(t, func() bool {
		return tests.Mana(t, peers[2]).Access > minAccessMana
	}, tests.Timeout, tests.Tick)
	log.Println("Request mana from faucet... done")

	// Test /mana
	t.Run("mana", func(t *testing.T) {
		// the faucet should have consensus mana and access mana = 1
		resp, err := faucet.GetManaFullNodeID(fullID(faucet.ID()))
		require.NoError(t, err)
		t.Logf("/mana %+v", resp)
		require.Equal(t, fullID(faucet.ID()), resp.NodeID)
		require.Greater(t, resp.Access, minAccessMana)

		// on startup, the faucet pledges consensus mana to the emptyNodeID
		require.Equal(t, resp.Consensus, minConsensusMana)
		resp, err = faucet.GetManaFullNodeID(fullID(emptyNodeID))
		require.NoError(t, err)
		t.Logf("/mana %+v", resp)
		require.Equal(t, fullID(emptyNodeID), resp.NodeID)
		require.Equal(t, 0.0, resp.Access)
		require.Greater(t, resp.Consensus, minConsensusMana)
	})

	// Test /mana/all
	t.Run("mana/all", func(t *testing.T) {
		resp, err := faucet.GetAllMana()
		require.NoError(t, err)
		t.Logf("/mana/all %+v", resp)
		require.NotEmpty(t, resp.Access)
		require.Greater(t, resp.Access[0].Mana, minAccessMana)
		require.NotEmpty(t, resp.Consensus)
		require.Greater(t, resp.Consensus[0].Mana, minConsensusMana)
	})

	// Test /mana/access/nhighest and /mana/consensus/nhighest
	t.Run("mana/*/nhighest", func(t *testing.T) {
		aResp, err := faucet.GetNHighestAccessMana(3)
		require.NoError(t, err)
		t.Logf("/mana/access/nhighest %+v", aResp)
		require.Len(t, aResp.Nodes, 3)
		prevMana := math.Inf(1)
		for i := range aResp.Nodes {
			require.LessOrEqual(t, aResp.Nodes[i].Mana, prevMana)
			prevMana = aResp.Nodes[i].Mana
		}

		expectedConsensusOrder := []identity.ID{peers[1].ID(), peers[2].ID(), peers[3].ID(), emptyNodeID}
		cResp, err := faucet.GetNHighestConsensusMana(len(expectedConsensusOrder))
		require.NoError(t, err)
		t.Logf("/mana/consensus/nhighest %+v", cResp)
		require.Len(t, cResp.Nodes, len(expectedConsensusOrder))
		for i := range expectedConsensusOrder {
			require.Equal(t, expectedConsensusOrder[i].String(), cResp.Nodes[i].ShortNodeID)
		}
	})

	// Test /mana/percentile
	t.Run("mana/percentile", func(t *testing.T) {
		resp, err := faucet.GetManaPercentile(fullID(peers[0].ID()))
		require.NoError(t, err)
		t.Logf("/mana/percentile %+v", resp)
		require.Equal(t, fullID(peers[0].ID()), resp.NodeID)
		require.InDelta(t, 75.0, resp.Access, 0.01)

		resp, err = faucet.GetManaPercentile(fullID(emptyNodeID))
		require.NoError(t, err)
		t.Logf("/mana/percentile %+v", resp)
		require.Equal(t, fullID(emptyNodeID), resp.NodeID)
		require.InDelta(t, 20., resp.Consensus, 0.01)
	})

	// Test /mana/access/online and /mana/consensus/online
	t.Run("mana/*/online", func(t *testing.T) {
		// genesis node is not online
		expectedOnlineAccessOrder := []string{peers[0].ID().String(), peers[1].ID().String(), peers[2].ID().String(), peers[3].ID().String()}
		aResp, err := faucet.GetOnlineAccessMana()
		require.NoError(t, err)
		t.Logf("/mana/access/online %+v", aResp)
		require.Len(t, aResp.Online, len(expectedOnlineAccessOrder))
		require.Equal(t, expectedOnlineAccessOrder[0], aResp.Online[0].ShortID)
		unorderedOnlineNodes := aResp.Online[1:]
		for j := range unorderedOnlineNodes {
			assert.Contains(t, expectedOnlineAccessOrder[1:], unorderedOnlineNodes[j].ShortID)
		}
		// empty node is not online
		expectedOnlineConsensusOrder := []identity.ID{peers[1].ID(), peers[2].ID(), peers[3].ID()}
		cResp, err := peers[0].GoShimmerAPI.GetOnlineConsensusMana()
		require.NoError(t, err)
		t.Logf("/mana/consensus/online %+v", cResp)
		require.Len(t, cResp.Online, len(expectedOnlineConsensusOrder))
		for i := range expectedOnlineConsensusOrder {
			require.Equal(t, expectedOnlineConsensusOrder[i].String(), cResp.Online[i].ShortID)
		}
	})

	// Test /mana/pending
	t.Run("mana/pending", func(t *testing.T) {
		unspentOutputs, err := peers[1].PostAddressUnspentOutputs([]string{peers[1].Address(0).Base58()})
		require.NoError(t, err)
		outputID := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.OutputID.Base58
		resp, err := peers[1].GetPending(outputID)
		require.NoError(t, err)
		t.Logf("/mana/pending %+v", resp)
		require.Equal(t, outputID, resp.OutputID)
		require.Greater(t, resp.Mana, minConsensusMana)
	})

	// Test /mana/allowedManaPledge
	t.Run("mana/allowedManaPledge", func(t *testing.T) {
		resp, err := faucet.GetAllowedManaPledgeNodeIDs()
		require.NoError(t, err)
		t.Logf("/mana/allowedManaPledge %+v", resp)
		require.Equal(t, false, resp.Access.IsFilterEnabled)
		require.Equal(t, []string{fullID(faucet.ID())}, resp.Access.Allowed)
		require.Equal(t, false, resp.Consensus.IsFilterEnabled)
		require.Equal(t, []string{fullID(faucet.ID())}, resp.Consensus.Allowed)
	})
}

func fullID(id identity.ID) string {
	return base58.Encode(id.Bytes())
}
