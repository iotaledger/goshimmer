package consensus

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/lo"
)

// constant var, shouldn't be changed
var tokensPerRequest int

const (
	// conflictRepetitions specify how many time we spam conflicts of each type
	conflictRepetitions = 4
	// numberOfConflictingOutputs is the number of outputs that will conflict for each tx we send.
	// Currently changing this value will require to change some implementation details
	numberOfConflictingOutputs = 3
	// splits - how many addresses we split the initial funds to.
	// For each conflict repetition we need to have numberOfConflictingOutputs addresses times number of conflict spam types (pairwise, triplets, and etc.)
	splits = conflictRepetitions * numberOfConflictingOutputs * 2
)

// TestConflictSpamAndMergeToMaster spams a node with conflicts and makes sure the confirmation states are the same across the network
// and verifies that the Tangle converged to Master
func TestConflictSpamAndMergeToMaster(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotOptions := tests.EqualSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		Faucet:      true,
		StartSynced: false,
		Activity:    false,
		Snapshot:    snapshotOptions,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	faucet, peer1 := n.Peers()[0], n.Peers()[1]
	tokensPerRequest = faucet.Config().TokensPerRequest

	const delayBetweenDataBlocks = 100 * time.Millisecond
	dataBlocksAmount := len(n.Peers()) * 10

	t.Logf("Sending %d data blocks to confirm Faucet Outputs", dataBlocksAmount)
	tests.SendDataBlocksWithDelay(t, n.Peers(), dataBlocksAmount, delayBetweenDataBlocks)

	fundingAddress := peer1.Address(0)
	tests.SendFaucetRequest(t, peer1, fundingAddress)

	t.Logf("Sending %d data blocks to confirm Faucet Funds", dataBlocksAmount)
	tests.SendDataBlocksWithDelay(t, n.Peers(), dataBlocksAmount, delayBetweenDataBlocks)

	require.Eventually(t, func() bool {
		return tests.Balance(t, peer1, fundingAddress, devnetvm.ColorIOTA) >= uint64(tokensPerRequest)
	}, tests.Timeout, tests.Tick)

	addresses := make([]devnetvm.Address, splits)
	keyPairs := make(map[string]*ed25519.KeyPair, splits)
	for i := 0; i < splits; i++ {
		address := peer1.Address(i)
		addresses[i] = address
		keyPairs[address.String()] = peer1.KeyPair(uint64(i))
	}

	outputs := getOutputsControlledBy(t, peer1, fundingAddress)
	fundingKeyPair := map[string]*ed25519.KeyPair{fundingAddress.String(): peer1.KeyPair(0)}
	outputs = splitToAddresses(t, peer1, outputs[0], fundingKeyPair, addresses...)

	// slice should have enough conflicting outputs for the number of loop repetition
	pairwiseOutputs := outputs[:conflictRepetitions*numberOfConflictingOutputs]
	tripletOutputs := outputs[len(pairwiseOutputs):]
	txs := make([]*devnetvm.Transaction, 0)
	for i := 0; i < conflictRepetitions; i++ {
		txs = append(txs, sendPairWiseConflicts(t, n.Peers(), determineOutputSlice(pairwiseOutputs, i, numberOfConflictingOutputs), keyPairs, i)...)
		txs = append(txs, sendTripleConflicts(t, n.Peers(), determineOutputSlice(tripletOutputs, i, numberOfConflictingOutputs), keyPairs, i)...)
	}

	t.Logf("Sending data %d blocks to confirm Conflicts and make ConfirmationState converge on all nodes", dataBlocksAmount*2)
	tests.SendDataBlocksWithDelay(t, n.Peers(), dataBlocksAmount*2, delayBetweenDataBlocks)

	t.Logf("number of txs to verify is %d", len(txs))
	verifyConfirmationsOnPeers(t, n.Peers(), txs)

	blkID, _ := tests.SendDataBlock(t, peer1, []byte("Gimme Master!"), 1)

	t.Logf("Verifying that %s is on MasterConflict", blkID)
	blockMetadata, err := peer1.GetBlockMetadata(blkID)
	require.NoError(t, err)
	require.Empty(t, blockMetadata.M.ConflictIDs)
}

// determineOutputSlice will extract sub-slices from outputs of a certain size.
// For each increment of i it will take the next sub-slice so there would be no overlaps with previous sub-slices.
func determineOutputSlice(outputs devnetvm.Outputs, i int, size int) devnetvm.Outputs {
	return outputs[i*size : i*size+size]
}

func verifyConfirmationsOnPeers(t *testing.T, peers []*framework.Node, txs []*devnetvm.Transaction) {
	const unknownConfirmationState = 10
	for _, tx := range txs {
		// current value signifies that we don't know what is the previous confirmation state
		var prevConfirmationState confirmation.State = unknownConfirmationState
		for i, peer := range peers {
			var metadata *jsonmodels.TransactionMetadata
			var err error
			require.Eventually(t, func() bool {
				metadata, err = peer.GetTransactionMetadata(tx.ID().Base58())
				return err == nil && metadata != nil
			}, 10*time.Second, 10*time.Millisecond, "Peer %s can't fetch metadata of tx %s. metadata is %v. Error is %w",
				peer.Name(), tx.ID().Base58(), metadata, err)
			t.Logf("ConfirmationState is %s for tx %s in peer %s", metadata.ConfirmationState, tx.ID().Base58(), peer.Name())
			if prevConfirmationState != unknownConfirmationState {
				require.Eventually(t,
					func() bool { return prevConfirmationState == metadata.ConfirmationState },
					10*time.Second, 10*time.Millisecond, "Different confirmation states on tx %s between peers %s (ConfirmationState=%s) and %s (ConfirmationState=%s)", tx.ID().Base58(),
					peers[i-1].Name(), prevConfirmationState, peer.Name(), metadata.ConfirmationState)
			}
			prevConfirmationState = metadata.ConfirmationState
		}
	}
}

// sendPairWiseConflicts receives a list of outputs controlled by a peer with certain peer index.
// It send them all to addresses controlled by the next peer, but it does so several time to create pairwise conflicts.
// The conflicts are TX_B<->TX_A<->TX_C
func sendPairWiseConflicts(t *testing.T, peers []*framework.Node, outputs devnetvm.Outputs,
	keyPairs map[string]*ed25519.KeyPair, iteration int,
) []*devnetvm.Transaction {
	t.Logf("send pairwise conflicts on iteration %d", iteration)
	peerIndex := (iteration + 1) % len(peers)

	// find target addresses
	targetAddresses := determineTargets(peers, iteration)

	tx1 := tests.CreateTransactionFromOutputs(t, peers[0].ID(), targetAddresses, keyPairs, outputs...)
	tx2 := tests.CreateTransactionFromOutputs(t, peers[1].ID(), targetAddresses, keyPairs, outputs[0])
	tx3 := tests.CreateTransactionFromOutputs(t, peers[2].ID(), targetAddresses, keyPairs, outputs[2])
	postTransactions(t, peers, peerIndex, "pairwise conflicts", tx1, tx2, tx3)

	return []*devnetvm.Transaction{tx1, tx2, tx3}
}

// Creates conflicts as so
// TX_A<->TX_B TX_B<->TX_C TX_C<->TX_A
func sendTripleConflicts(t *testing.T, peers []*framework.Node, outputs devnetvm.Outputs,
	keyPairs map[string]*ed25519.KeyPair, iteration int,
) []*devnetvm.Transaction {
	t.Logf("send triple conflicts on iteration %d", iteration)

	peerIndex := (iteration + 1) % len(peers)

	// find target addresses
	targetAddresses := determineTargets(peers, iteration)

	tx1 := tests.CreateTransactionFromOutputs(t, peers[0].ID(), targetAddresses, keyPairs, outputs...)
	tx2 := tests.CreateTransactionFromOutputs(t, peers[1].ID(), targetAddresses, keyPairs, outputs[0], outputs[1])
	tx3 := tests.CreateTransactionFromOutputs(t, peers[2].ID(), targetAddresses, keyPairs, outputs[1], outputs[2])
	postTransactions(t, peers, peerIndex, "triplet conflicts", tx1, tx2, tx3)

	return []*devnetvm.Transaction{tx1, tx2, tx3}
}

func postTransactions(t *testing.T, peers []*framework.Node, peerIndex int, attackName string, txs ...*devnetvm.Transaction) {
	for i, tx := range txs {
		newPeerIndex := (peerIndex + i) % len(peers)
		log.Printf("%s: post tx %s on peer %s", attackName, tx.ID().Base58(), peers[newPeerIndex].Name())
		resp, err := peers[newPeerIndex].PostTransaction(lo.PanicOnErr(tx.Bytes()))
		require.NoError(t, err, "%s: There was an error posting transaction %s to peer %s",
			attackName, tx.ID().Base58(), peers[newPeerIndex].Name())
		require.Empty(t, resp.Error, "%s: There was an error in the response while posting transaction %s to peer %s",
			attackName, tx.ID().Base58(), peers[newPeerIndex].Name())
		time.Sleep(50 * time.Millisecond)
	}
}

func determineTargets(peers []*framework.Node, index int) []devnetvm.Address {
	targetIndex := (index + 1) % len(peers)
	targetPeer := peers[targetIndex]
	targetAddresses := make([]devnetvm.Address, 0)

	for i := index * numberOfConflictingOutputs; i < index*numberOfConflictingOutputs+numberOfConflictingOutputs; i++ {
		targetAddress := targetPeer.Address(i)
		targetAddresses = append(targetAddresses, targetAddress)
	}
	return targetAddresses
}

func getOutputsControlledBy(t *testing.T, node *framework.Node, addresses ...devnetvm.Address) devnetvm.Outputs {
	outputs := devnetvm.Outputs{}
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

func splitToAddresses(t *testing.T, node *framework.Node, output devnetvm.Output, keyPairs map[string]*ed25519.KeyPair, addresses ...devnetvm.Address) devnetvm.Outputs {
	transaction := tests.CreateTransactionFromOutputs(t, node.ID(), addresses, keyPairs, output)
	_, err := node.PostTransaction(lo.PanicOnErr(transaction.Bytes()))
	require.NoError(t, err, "Error occured while trying to split addresses")
	return transaction.Essence().Outputs()
}
