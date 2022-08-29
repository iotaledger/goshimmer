package mana

import (
	"context"
	"log"
	"math"
	"testing"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

var (
	// minAccessMana is minimal amout of mana required to access the network
	minAccessMana      = tangleold.MinMana
	startAccessMana    = 2_500_000_000_000_000.
	faucetPledgeAmount = 1_000_000.
	minConsensusMana   = 0.0
)

//
// func TestManaPersistence(t *testing.T) {
//	ctx, cancel := tests.Context(context.Background(), t)
//	defer cancel()
//	snapshotInfo := tests.EqualSnapshotDetails
//	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
//		Faucet:      true,
//		StartSynced: true,
//		Activity:    true,
//		PeerMaster:  true,
//		Snapshot:    snapshotInfo,
//	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
//	require.NoError(t, err)
//	defer tests.ShutdownNetwork(ctx, t, n)
//
//	faucet, peer := n.Peers()[0], n.Peers()[1]
//
//	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())
//	tests.SendFaucetRequest(t, peer, peer.Address(0))
//
//	log.Println("Waiting for peer to get access mana...")
//	require.Eventually(t, func() bool {
//		return tests.Mana(t, peer).Access > minAccessMana
//	}, tests.Timeout, tests.Tick)
//	log.Println("Waiting for peer to get consensus mana...")
//	require.Eventually(t, func() bool {
//		return tests.Mana(t, peer).Consensus > 0
//	}, tests.Timeout, tests.Tick)
//	log.Println("Waiting for peer to get mana... done")
//
//	// restart the peer
//	require.NoError(t, peer.Restart(ctx))
//
//	require.Greater(t, tests.Mana(t, peer).Access, minAccessMana)
//	require.Greater(t, tests.Mana(t, peer).Consensus, minConsensusMana)
// }
//
// func TestManaPledgeFilter(t *testing.T) {
//	const (
//		numPeers         = 3
//		tokensPerRequest = 100
//	)
//	snapshotInfo := tests.EqualSnapshotDetails
//	ctx, cancel := tests.Context(context.Background(), t)
//	defer cancel()
//	n, err := f.CreateNetwork(ctx, t.Name(), numPeers, framework.CreateNetworkConfig{
//		StartSynced: true,
//		Activity:    true,
//		PeerMaster:  false,
//		Snapshot:    snapshotInfo,
//	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
//	require.NoError(t, err)
//	defer tests.ShutdownNetwork(ctx, t, n)
//
//	peers := n.Peers()
//
//	accessPeer := peers[0]
//	accessPeerID := fullID(accessPeer.ID())
//	consensusPeer := peers[1]
//	consensusPeerID := fullID(consensusPeer.ID())
//	seedBytes, err := base58.Decode(snapshotInfo.MasterSeed)
//	require.NoError(t, err)
//
//	faucetConfig := framework.PeerConfig()
//	faucetConfig.BlockLayer.StartSynced = true
//	faucetConfig.Faucet.Enabled = true
//	faucetConfig.Mana.Enabled = true
//	faucetConfig.Mana.AllowedAccessPledge = []string{accessPeerID}
//	faucetConfig.Mana.AllowedAccessFilterEnabled = true
//	faucetConfig.Mana.AllowedConsensusPledge = []string{consensusPeerID}
//	faucetConfig.Mana.AllowedConsensusFilterEnabled = true
//	faucetConfig.Activity.Enabled = true
//	faucetConfig.Seed = seedBytes
//	faucetConfig.BlockLayer.Snapshot.File = snapshotInfo.FilePath
//
//	faucet, err := n.CreatePeer(ctx, faucetConfig)
//	require.NoError(t, err)
//
//	err = n.DoManualPeering(ctx)
//	require.NoError(t, err)
//
//	// wait for the faucet to prepare all outputs
//	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())
//
//	var faucetStartAddress uint64 = faucetplugin.MaxFaucetOutputsCount + 1
//	// pledge mana to allowed peers
//	_, err = tests.SendTransaction(t, faucet, accessPeer, devnetvm.ColorIOTA, tokensPerRequest, tests.TransactionConfig{
//		FromAddressIndex:      faucetStartAddress,
//		ToAddressIndex:        0,
//		AccessManaPledgeID:    accessPeer.Identity.ID(),
//		ConsensusManaPledgeID: consensusPeer.Identity.ID(),
//	})
//	require.NoError(t, err)
//
//	// pledge consensus mana to forbidden peer
//	_, err = tests.SendTransaction(t, faucet, accessPeer, devnetvm.ColorIOTA, tokensPerRequest, tests.TransactionConfig{
//		FromAddressIndex:      faucetStartAddress,
//		ToAddressIndex:        0,
//		AccessManaPledgeID:    accessPeer.Identity.ID(),
//		ConsensusManaPledgeID: accessPeer.Identity.ID(),
//	})
//	require.Contains(t, err.Error(), webapiledgerstate.ErrNotAllowedToPledgeManaToNode.Error())
//
//	// pledge access mana to forbidden peer
//	_, err = tests.SendTransaction(t, faucet, accessPeer, devnetvm.ColorIOTA, tokensPerRequest, tests.TransactionConfig{
//		FromAddressIndex:      faucetStartAddress,
//		ToAddressIndex:        0,
//		AccessManaPledgeID:    consensusPeer.Identity.ID(),
//		ConsensusManaPledgeID: consensusPeer.Identity.ID(),
//	})
//	require.ErrorIs(t, err, client.ErrBadRequest)
//	require.Contains(t, err.Error(), webapiledgerstate.ErrNotAllowedToPledgeManaToNode.Error())
// }

func TestManaApis(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotInfo := tests.EqualSnapshotDetails
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Autopeering: true, // we need to discover online peers
		Activity:    true, // we need to issue regular activity blocks
		PeerMaster:  true,
		Snapshot:    snapshotInfo,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	peers := n.Peers()
	faucet := peers[0]

	log.Println("Request mana from faucet...")
	// waiting for the faucet to have access mana
	require.Eventually(t, func() bool {
		return tests.Mana(t, faucet).Access > minAccessMana
	}, tests.Timeout, tests.Tick)

	// request mana for peer #1; do this twice to assure that peer #1 gets more mana than peer #2
	tests.SendFaucetRequest(t, peers[1], peers[1].Address(0))
	tests.SendFaucetRequest(t, peers[1], peers[1].Address(1))

	require.Eventually(t, func() bool {
		return tests.Mana(t, peers[1]).Access >= startAccessMana+2*faucetPledgeAmount
	}, tests.Timeout, tests.Tick)

	// request mana for peer #2
	tests.SendFaucetRequest(t, peers[2], peers[2].Address(0))

	require.Eventually(t, func() bool {
		return tests.Mana(t, peers[2]).Access >= startAccessMana+faucetPledgeAmount
	}, tests.Timeout, tests.Tick)

	// Test /mana/all
	t.Run("mana/all access", func(t *testing.T) {
		respAll, err := faucet.GetAllMana()
		require.NoError(t, err)
		t.Logf("/mana/all %+v", respAll)
		require.NotEmpty(t, respAll.Access)
		require.Greater(t, respAll.Access[0].Mana, minAccessMana)
	})

	// Test /mana/access/nhighest and /mana/consensus/nhighest
	t.Run("mana/*/nhighest access", func(t *testing.T) {
		aResp, err := faucet.GetNHighestAccessMana(len(n.Peers()))
		require.NoError(t, err)
		t.Logf("/mana/access/nhighest %+v", aResp)
		require.Len(t, aResp.Nodes, len(n.Peers()))
		prevMana := math.Inf(1)
		for i := range aResp.Nodes {
			require.LessOrEqual(t, aResp.Nodes[i].Mana, prevMana)
			prevMana = aResp.Nodes[i].Mana
		}
	})

	// Test /mana/percentile
	t.Run("mana/percentile access", func(t *testing.T) {
		percResp, err := faucet.GetManaPercentile(fullID(faucet.ID()))
		require.NoError(t, err)
		t.Logf("/mana/percentile %+v", percResp)
		require.Equal(t, fullID(faucet.ID()), percResp.NodeID)
		require.InDelta(t, 0, percResp.Access, 0.01)
	})

	// Test /mana/access/online
	t.Run("mana/*/online access", func(t *testing.T) {
		// genesis node is not online
		aResp, err := peers[0].GoShimmerAPI.GetOnlineAccessMana()
		require.NoError(t, err)
		t.Logf("/mana/access/online %+v", aResp)
		require.Len(t, aResp.Online, len(n.Peers()))
		nodeIDs := make([]string, len(aResp.Online))
		for i := range aResp.Online {
			nodeIDs[i] = aResp.Online[i].ID
		}
		require.Len(t, nodeIDs, len(n.Peers()))
		for _, peer := range n.Peers() {
			require.Contains(t, nodeIDs, peer.ID().EncodeBase58())
		}
	})

	// Test /mana/allowedManaPledge
	t.Run("mana/allowedManaPledge access", func(t *testing.T) {
		respAll, err := faucet.GetAllowedManaPledgeNodeIDs()
		require.NoError(t, err)
		t.Logf("/mana/allowedManaPledge %+v", respAll)
		require.Equal(t, false, respAll.Access.IsFilterEnabled)
		require.Equal(t, []string{fullID(faucet.ID())}, respAll.Access.Allowed)
	})

	// wait for cMna vector being updated
	require.Eventually(t, func() bool {
		return tests.Mana(t, peers[1]).Consensus > tests.Mana(t, peers[2]).Consensus
	}, tests.Timeout*2, tests.Tick)

	// Test /mana/all
	t.Run("mana/all consensus", func(t *testing.T) {
		respAll, err := faucet.GetAllMana()
		require.NoError(t, err)
		t.Logf("/mana/all %+v", respAll)
		require.NotEmpty(t, respAll.Consensus)
		require.Greater(t, respAll.Consensus[0].Mana, minConsensusMana)
	})

	// Test /mana/access/nhighest and /mana/consensus/nhighest
	t.Run("mana/*/nhighest consensus", func(t *testing.T) {
		cResp, err := faucet.GetNHighestConsensusMana(len(n.Peers()))
		require.NoError(t, err)
		t.Logf("/mana/consensus/nhighest %+v", cResp)
		nodeIDs := make([]string, len(cResp.Nodes))
		for i := range cResp.Nodes {
			nodeIDs[i] = cResp.Nodes[i].NodeID
		}
		require.Len(t, nodeIDs, len(n.Peers()))
		for _, peer := range n.Peers() {
			// the faucet does not have any consensus mana.
			if peer.ID() == faucet.ID() {
				continue
			}
			require.Contains(t, nodeIDs, peer.ID().EncodeBase58())
		}

		require.NotContains(t, nodeIDs, faucet.ID)
		require.Contains(t, nodeIDs, identity.ID{}.EncodeBase58())
	})

	// Test /mana/percentile
	t.Run("mana/percentile", func(t *testing.T) {
		respPerc, err := faucet.GetManaPercentile(faucet.ID().EncodeBase58())
		require.NoError(t, err)
		t.Logf("/mana/percentile %+v", respPerc)
		require.Equal(t, faucet.ID().EncodeBase58(), respPerc.NodeID)
		require.InDelta(t, 0, respPerc.Consensus, 0.01)
	})

	// Test /mana/consensus/online
	t.Run("mana/*/online consensus", func(t *testing.T) {
		cResp, err := peers[0].GoShimmerAPI.GetOnlineConsensusMana()
		require.NoError(t, err)
		t.Logf("/mana/consensus/online %+v", cResp)
		// the faucet does not have any consensus mana.
		require.Len(t, cResp.Online, len(n.Peers())-1)
		nodeIDs := make([]string, len(cResp.Online))
		for i := range cResp.Online {
			nodeIDs[i] = cResp.Online[i].ID
		}
		for _, peer := range n.Peers() {
			// the faucet does not have any consensus mana.
			if peer.ID() == faucet.ID() {
				continue
			}
			require.Contains(t, nodeIDs, peer.ID().EncodeBase58())
		}
		require.NotContains(t, nodeIDs, faucet.ID)
	})

	// Test /mana/allowedManaPledge
	t.Run("mana/allowedManaPledge consensus", func(t *testing.T) {
		respAll, err := faucet.GetAllowedManaPledgeNodeIDs()
		require.NoError(t, err)
		t.Logf("/mana/allowedManaPledge %+v", respAll)
		require.Equal(t, false, respAll.Consensus.IsFilterEnabled)
		require.Equal(t, []string{fullID(faucet.ID())}, respAll.Consensus.Allowed)
	})
}

func fullID(id identity.ID) string {
	return base58.Encode(id.Bytes())
}
