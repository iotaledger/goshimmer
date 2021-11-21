package consensus

import (
	"context"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
)

// constant var, shouldn't be changed
var tokensPerRequest int

/**
TestConflictSpam spams a node with conflicts and makes sure the GoFs are the same across the network
*/
func TestConflictSpam(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		Faucet:      true,
		StartSynced: true,
		Activity:    true,
	}, tests.EqualDefaultConfigFunc(t, false))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet, peer1 := n.Peers()[0], n.Peers()[1]
	tokensPerRequest = faucet.Config().TokensPerRequest

	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	fundingAddress := peer1.Address(0)
	tests.SendFaucetRequest(t, peer1, fundingAddress)
	require.Eventually(t, func() bool {
		return tests.Balance(t, peer1, fundingAddress, ledgerstate.ColorIOTA) >= uint64(tokensPerRequest)
	}, tests.Timeout, tests.Tick)

	addresses := make([]*ledgerstate.Address, 100)
	keyPairs := map[string]*ed25519.KeyPair{}
	for i := 0; i < 100; i++ {
		address := peer1.Address(i)
		addresses[i] = &address
		keyPairs[address.String()] = peer1.KeyPair(uint64(i))
	}

	outputs := getOutputsControlledBy(t, peer1, fundingAddress)
	fundingKeyPair := map[string]*ed25519.KeyPair{fundingAddress.String(): peer1.KeyPair(0)}
	outputs = splitToAddresses(t, peer1, outputs[0], fundingKeyPair, addresses...)
	const conflictRepetitions = 4
	pairwiseOutputs := outputs[:(conflictRepetitions-1)*3+3]
	tripletOutputs := outputs[len(pairwiseOutputs):]

	txs := []*ledgerstate.Transaction{}
	for i := 0; i < conflictRepetitions; i++ {
		sendPairWiseConflicts(t, n.Peers(), pairwiseOutputs[i*3:i*3+3], keyPairs, &txs, i)
		sendTripleConflicts(t, n.Peers(), tripletOutputs[i*3:i*3+3], keyPairs, &txs, i)
	}
	t.Logf("number of txs to verify is %d", len(txs))
	verifyConfirmationsOnPeers(t, n.Peers(), txs)
}

func verifyConfirmationsOnPeers(t *testing.T, peers []*framework.Node, txs []*ledgerstate.Transaction) {
	const unknownGoF = 10
	for _, tx := range txs {
		// current value signifies that we don't know what is the previous gof
		var prevGoF gof.GradeOfFinality = unknownGoF
		for i, peer := range peers {
			var metadata *jsonmodels.TransactionMetadata
			var err error
			require.Eventually(t, func() bool {
				metadata, err = peer.GetTransactionMetadata(tx.ID().Base58())
				return err == nil && metadata != nil
			}, 10*time.Second, time.Second, "Peer %s can't fetch metadata of tx %s. metadata is %v. Error is %w",
				peer.Name(), tx.ID().Base58(), metadata, err)
			t.Logf("GoF is %d for tx %s in peer %s", metadata.GradeOfFinality, tx.ID().Base58(), peer.Name())
			if prevGoF != unknownGoF {
				require.EqualValues(t, prevGoF, metadata.GradeOfFinality,
					"Different gofs on tx %s between peers %s and %s", tx.ID().Base58(),
					peers[i-1].Name(), peer.Name())
			}
			prevGoF = metadata.GradeOfFinality
		}
	}
}

/**
sendPairWiseConflicts receives a list of outputs controlled by a peer with certain peer index.
It send them all to addresses controlled by the next peer, but it does so several time to create pairwise conflicts.
The conflicts are TX_B<->TX_A<->TX_C
*/
func sendPairWiseConflicts(t *testing.T, peers []*framework.Node, outputs ledgerstate.Outputs,
	keyPairs map[string]*ed25519.KeyPair,
	txs *[]*ledgerstate.Transaction, iteration int) {

	t.Logf("send pairwise conflicts on iteration %d", iteration)
	peerIndex := (iteration + 1) % len(peers)

	// find target addresses
	targetAddresses := determineTargets(peers, iteration)

	tx1 := tests.CreateTransactionFromOutputs(t, peers[0].ID(), targetAddresses, keyPairs, outputs...)
	tx2 := tests.CreateTransactionFromOutputs(t, peers[1].ID(), targetAddresses, keyPairs, outputs[0])
	tx3 := tests.CreateTransactionFromOutputs(t, peers[2].ID(), targetAddresses, keyPairs, outputs[2])

	*txs = append(*txs, tx1, tx2, tx3)

	PostTransactions(t, peers, peerIndex, "pairwise conflicts", tx1, tx2, tx3)
}

/**
Creates conflicts as so
TX_A<->TX_B TX_B<->TX_C TX_C<->TX_A
*/
func sendTripleConflicts(t *testing.T, peers []*framework.Node, outputs ledgerstate.Outputs,
	keyPairs map[string]*ed25519.KeyPair,
	txs *[]*ledgerstate.Transaction, iteration int) {
	t.Logf("send triple conflicts on iteration %d", iteration)

	peerIndex := (iteration + 1) % len(peers)

	// find target addresses
	targetAddresses := determineTargets(peers, iteration)

	tx1 := tests.CreateTransactionFromOutputs(t, peers[0].ID(), targetAddresses, keyPairs, outputs...)
	tx2 := tests.CreateTransactionFromOutputs(t, peers[1].ID(), targetAddresses, keyPairs, outputs[0], outputs[1])
	tx3 := tests.CreateTransactionFromOutputs(t, peers[2].ID(), targetAddresses, keyPairs, outputs[1], outputs[2])

	*txs = append(*txs, tx1, tx2, tx3)

	PostTransactions(t, peers, peerIndex, "triplet conflicts", tx1, tx2, tx3)
}

func PostTransactions(t *testing.T, peers []*framework.Node, peerIndex int, attackName string, txs ...*ledgerstate.Transaction) {
	for i, tx := range txs {
		newPeerIndex := (peerIndex + i) % len(peers)
		resp, err := peers[newPeerIndex].PostTransaction(tx.Bytes())
		t.Logf("%s: post tx %s on peer %s", attackName, tx.ID().Base58(), peers[peerIndex].Name())
		require.NoError(t, err, "%s: There was an error posting transaction %s to peer %s",
			attackName, tx.ID().Base58(), peers[peerIndex].Name())
		require.Empty(t, resp.Error, "%s: There was an error in the response while posting transaction %s to peer %s",
			attackName, tx.ID().Base58(), peers[peerIndex].Name())
		time.Sleep(time.Second)
	}
}

func determineTargets(peers []*framework.Node, iteration int) []*ledgerstate.Address {
	targetIndex := (iteration + 1) % len(peers)
	targetPeer := peers[targetIndex]
	targetAddresses := []*ledgerstate.Address{}

	for i := iteration * 3; i < iteration*3+3; i++ {
		targetAddress := targetPeer.Address(i)
		targetAddresses = append(targetAddresses, &targetAddress)
	}
	return targetAddresses
}

func getOutputsControlledBy(t *testing.T, node *framework.Node, addresses ...ledgerstate.Address) ledgerstate.Outputs {
	outputs := ledgerstate.Outputs{}
	for _, address := range addresses {
		walletOutputs := tests.AddressUnspentOutputs(t, node, address, 1)
		for _, walletOutput := range walletOutputs {
			t.Logf("wallet output is %v", walletOutput)
			output, err := walletOutput.Output.ToLedgerstateOutput()
			require.NoError(t, err, "Failed to convert output to ledgerstate output")
			outputs = append(outputs, output)
		}
	}
	return outputs
}

func splitToAddresses(t *testing.T, node *framework.Node, output ledgerstate.Output, keyPairs map[string]*ed25519.KeyPair, addresses ...*ledgerstate.Address) ledgerstate.Outputs {
	transaction := tests.CreateTransactionFromOutputs(t, node.ID(), addresses, keyPairs, output)
	_, err := node.PostTransaction(transaction.Bytes())
	require.NoError(t, err, "Error occured while trying to split addresses")
	return transaction.Essence().Outputs()
}
