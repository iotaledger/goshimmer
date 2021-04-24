package mana

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	manaPkg "github.com/iotaledger/goshimmer/packages/mana"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
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
	// cons mana is pledged to emptyNodeID
	require.Equal(t, manaBefore.Consensus, 0.0)

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
	require.Equal(t, manaAfter.Consensus, 0.0)
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
	assert.Equal(t, resp.Access, 0.0)
	assert.Greater(t, resp.Consensus, 0.0)

	// access mana was pledged to itself by the faucet
	resp, err = peers[0].GoShimmerAPI.GetManaFullNodeID(base58.Encode(peers[0].ID().Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp.NodeID)
	assert.Greater(t, resp.Access, 0.0)
	assert.Equal(t, resp.Consensus, 0.0)

	// Test /mana/all
	resp2, err := peers[0].GoShimmerAPI.GetAllMana()
	require.NoError(t, err)
	assert.Equal(t, 3, len(resp2.Access))
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
	timestampPast := allManaResp.ConsensusTimestamp
	resp3, err := peers[0].GoShimmerAPI.GetNHighestAccessMana(len(peers) + 2)
	require.NoError(t, err)
	resp3.Nodes = stripGenesisNodeID(resp3.Nodes)
	resp4, err := peers[0].GoShimmerAPI.GetNHighestConsensusMana(len(peers) + 2)
	require.NoError(t, err)
	resp4.Nodes = stripGenesisNodeID(resp4.Nodes)
	require.Equal(t, 3, len(resp3.Nodes))
	require.Equal(t, 3, len(resp4.Nodes))
	for i := 0; i < 3; i++ {
		assert.Equal(t, base58.Encode(peers[i].ID().Bytes()), resp3.Nodes[i].NodeID)
		// faucet pledged its cons mana to emptyNodeID...
		if i == 0 {
			assert.Equal(t, base58.Encode(emptyNodeID.Bytes()), resp4.Nodes[i].NodeID)
		} else {
			assert.Equal(t, base58.Encode(peers[i].ID().Bytes()), resp4.Nodes[i].NodeID)
		}
	}

	// Test /mana/percentile
	resp5, err := peers[0].GoShimmerAPI.GetManaPercentile(base58.Encode(peers[0].ID().Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp5.NodeID)
	assert.InDelta(t, 80.0, resp5.Access, 0.01)

	resp5, err = peers[0].GoShimmerAPI.GetManaPercentile(base58.Encode(emptyNodeID.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, base58.Encode(emptyNodeID.Bytes()), resp5.NodeID)
	assert.InDelta(t, 40., resp5.Consensus, 0.01)

	// Test /mana/online/access
	resp6, err := peers[0].GoShimmerAPI.GetOnlineAccessMana()
	require.NoError(t, err)
	resp7, err := peers[0].GoShimmerAPI.GetOnlineConsensusMana()
	require.NoError(t, err)
	require.Equal(t, 3, len(resp6.Online))
	// emptyNodeID cannot be online!
	require.Equal(t, 2, len(resp7.Online))
	fmt.Println("online nodes access mana")
	for _, r := range resp6.Online {
		fmt.Println("node - ", r.ShortID, " -- mana: ", r.Mana)
	}
	assert.Equal(t, base58.Encode(peers[0].ID().Bytes()), resp6.Online[0].ID)
	assert.Equal(t, base58.Encode(peers[1].ID().Bytes()), resp6.Online[1].ID)
	assert.Equal(t, base58.Encode(peers[2].ID().Bytes()), resp6.Online[2].ID)

	// emptyNodeID cannot be online!
	assert.Equal(t, base58.Encode(peers[1].ID().Bytes()), resp7.Online[0].ID)
	assert.Equal(t, base58.Encode(peers[2].ID().Bytes()), resp7.Online[1].ID)

	// Test /mana/pending
	unspentOutputs, err := peers[1].GetUnspentOutputs([]string{peers[1].Seed.Address(0).Address().Base58()})
	require.NoError(t, err)
	outputID := unspentOutputs.UnspentOutputs[0].OutputIDs[0].ID
	resp8, err := peers[1].GetPending(outputID)
	require.NoError(t, err)
	assert.Equal(t, outputID, resp8.OutputID)
	fmt.Println("pending mana: ", resp8.Mana)
	assert.Greater(t, resp8.Mana, 0.0)

	// Test/mana/consensus/past
	// send funds to node 3 to trigger more consensus events.
	time.Sleep(5 * time.Second) // we wait a bit to not overlap with timestampPast
	_, err = peers[3].SendFaucetRequest(peers[3].Seed.Address(0).Address().Base58())
	require.NoError(t, err)
	time.Sleep(12 * time.Second)
	resp9, err := peers[0].GoShimmerAPI.GetPastConsensusManaVector(timestampPast)
	require.NoError(t, err)
	assert.Equal(t, 5, len(resp9.Consensus)) //excluding node 3
	m := make(map[string]float64)
	for _, c := range resp9.Consensus {
		m[c.ShortNodeID] = c.Mana
	}

	mana, ok := m[emptyNodeID.String()]
	assert.True(t, ok)
	assert.Greater(t, mana, 0.0)
	// node 3 shouldn't have mana from way back at `timestampPast`
	for _, p := range peers[1:3] {
		mana, ok := m[p.ID().String()]
		assert.True(t, ok)
		assert.Greater(t, mana, 0.0)
	}

	// Test /mana/consensus/logs
	resp10, err := peers[0].GoShimmerAPI.GetConsensusEventLogs([]string{})
	require.NoError(t, err)
	fmt.Println("consensus mana event logs")
	for n, l := range resp10.Logs {
		fmt.Println("node: ", n, " pledge logs: ", len(l.Pledge), " revoke logs: ", len(l.Revoke))
	}

	// emptyNodeID was pledged once (splitting genesis) and revoked 3 (3 requests)
	logs, ok := resp10.Logs[base58.Encode(emptyNodeID.Bytes())]
	require.True(t, ok)
	assert.Equal(t, 1, len(logs.Pledge))
	assert.Equal(t, 3, len(logs.Revoke))

	for i := 1; i < len(peers); i++ {
		logs, ok := resp10.Logs[base58.Encode(peers[i].ID().Bytes())]
		require.True(t, ok)
		assert.Equal(t, 1, len(logs.Pledge))
		assert.Equal(t, 0, len(logs.Revoke))
	}

	// Test /mana/consensus/pastmetadata
	err = peers[0].Stop()
	require.NoError(t, err)
	err = peers[0].Start()
	require.NoError(t, err)
	time.Sleep(3 * time.Second)
	_, err = peers[0].GoShimmerAPI.GetPastConsensusVectorMetadata()
	require.NoError(t, err)
}

func stripGenesisNodeID(input []manaPkg.NodeStr) (output []manaPkg.NodeStr) {
	peerMaster := "2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"
	faucet := "FZ6xmPZXRs2M8z9m9ETTQok4PCga4X8FRHwQE6uYm4rV"
	for _, id := range input {
		if id.NodeID == peerMaster || id.NodeID == faucet {
			continue
		}
		output = append(output, id)
	}
	return
}
