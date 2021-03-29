package consensus

import (
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
	"log"
	"testing"
	"time"

	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/identity"
)

// TestConsensusFiftyFiftyOpinionSplit spawns two network partitions with their own peers,
// then issues valid value objects spending the genesis in both, deletes the partitions (and lets them merge)
// and then checks that the conflicts are resolved via FPC.
func TestConsensusFiftyFiftyOpinionSplit(t *testing.T) {

	// override avg. network delay to accustom integration test slowness
	backupFCoBAvgNetworkDelay := framework.ParaFCoBAverageNetworkDelay
	backupBootstrapOnEveryNode := framework.ParaSyncBeaconOnEveryNode
	backupParaWaitToKill := framework.ParaWaitToKill
	framework.ParaFCoBAverageNetworkDelay = 120
	framework.ParaSyncBeaconOnEveryNode = true
	framework.ParaWaitToKill = 2*framework.ParaFCoBAverageNetworkDelay + 10

	const numberOfPeers = 6

	// reset framework paras
	defer func() {
		framework.ParaFCoBAverageNetworkDelay = backupFCoBAvgNetworkDelay
		framework.ParaSyncBeaconOnEveryNode = backupBootstrapOnEveryNode
		framework.ParaWaitToKill = backupParaWaitToKill
	}()

	// create two partitions with their own peers
	n, err := f.CreateNetworkWithPartitions("abc", numberOfPeers, 2, 2, framework.CreateNetworkConfig{})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	// split the network
	for i, partition := range n.Partitions() {
		log.Printf("partition %d peers:", i)
		for _, p := range partition.Peers() {
			log.Println(p.ID().String())
		}
	}

	// genesis wallet
	genesisSeedBytes, err := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
	require.NoError(t, err, "couldn't decode genesis seed from base58 seed")

	// make genesis fund easily divisible for further splitting of the funds
	const genesisBalance = (numberOfPeers + 1) * 100000000000000
	genesisSeed := seed.NewSeed(genesisSeedBytes)
	genesisOutputID := ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0)
	input := ledgerstate.NewUTXOInput(genesisOutputID)
	// splitting genesis funds to one address per peer plus one additional that will be used for the conflict
	spendingGenTx, destGenSeed := CreateOutputs(input, genesisBalance, genesisSeed, numberOfPeers + 1, identity.ID{})

	// issue the transaction on the first peer of each partition, both partitions will have the same view
	issueTransaction(n.Partitions()[0].Peers()[0], spendingGenTx, t, "genesis splitting")
	issueTransaction(n.Partitions()[1].Peers()[0], spendingGenTx, t, "genesis splitting")

	// sleep the avg. network delay so both partitions confirm their own first seen transaction
	log.Printf("waiting %d seconds avg. network delay to make the transactions "+
		"preferred in their corresponding partition", framework.ParaFCoBAverageNetworkDelay)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay) * time.Second)

	// issue one transaction per peer to pledge mana to nodes
	// leave one unspent output from splitting genesis transaction for further conflict creation

	//prepare all the pledgingTxs
	pledgingTxs := make([]*ledgerstate.Transaction, numberOfPeers + 1)
	pledgeSeed := make([]*seed.Seed, numberOfPeers + 1)
	receiverId := 0
	for _, partition := range n.Partitions() {
		for _, peer := range partition.Peers() {
			// get next dest addresses
			destAddr := destGenSeed.Address(uint64(receiverId)).Address()
			receiverId++
			// Get tx output for current dest address
			outputGenTx := spendingGenTx.Essence().Outputs().Filter(func (output ledgerstate.Output) bool {
				return output.Address() == destAddr
			})[0]
			balance := outputGenTx.Balances().Size()  //TODO does it return what I want?
			pledgeInput := ledgerstate.NewUTXOInput(outputGenTx.ID())
			pledgingTxs[receiverId], pledgeSeed[receiverId] = CreateOutputs(pledgeInput, balance, destGenSeed, 1, peer.ID())  //TODO is it correct that I put here destSeed

			// issue the transaction to peers on both partitions
			issueTransaction(n.Partitions()[0].Peers()[0], pledgingTxs[receiverId], t, "pledging")
			issueTransaction(n.Partitions()[1].Peers()[0], pledgingTxs[receiverId], t, "pledging")
		}
	}
	// wait 3 times network delay
	// sleep 2* the avg. network delay so both partitions confirm their own pledging transaction
	// and 1 avg delay more to make sure each node has mana
	log.Printf("waiting 3 * %d seconds avg. network delay to make the transactions "+
		"preferred in their corresponding partition", framework.ParaFCoBAverageNetworkDelay)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay) * 3 * time.Second)

	//prepare two conflicting transactions from one additional unused genesis output
	conflictingTxs := make([]*ledgerstate.Transaction, len(n.Partitions()))
	conflictingTxIDs := make([]string, len(n.Partitions()))
	receiverSeeds := make([]*seed.Seed, len(n.Partitions()))
	// get address for last created unused genesis output
	lastAddress := destGenSeed.Address(uint64(numberOfPeers - 1)).Address()
	// Get ast created unused genesis output and its balance
	lastOutputTx := spendingGenTx.Essence().Outputs().Filter(func (output ledgerstate.Output) bool {
		return output.Address() == lastAddress
	})[0]
	lastOutputBalance := lastOutputTx.Balances().Size()
	// prepare two conflicting transactions, one per partition
	for i, partition := range n.Partitions() {
		partitionReceiverSeed := seed.NewSeed()
		conflictInput := ledgerstate.NewUTXOInput(lastOutputTx.ID())
		conflictingTxs[i], receiverSeeds[i] = CreateOutputs(conflictInput, lastOutputBalance, partitionReceiverSeed, 1, partition.Peers()[0].ID())

		// issue conflicting transaction on the current partition
		txId := issueTransaction(partition.Peers()[0], conflictingTxs[i], t, "conflicting")
		conflictingTxIDs[i] = txId
	}

	// sleep the avg. network delay so both partitions prefer their own first seen transaction
	log.Printf("waiting 3* %d seconds avg. network delay to make the transactions "+
		"preferred in their corresponding partition", framework.ParaFCoBAverageNetworkDelay)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay) * 3 * time.Second)

	// check that each partition is preferring its corresponding transaction
	log.Println("checking that each partition likes its corresponding transaction before the conflict:")
	for i, partition := range n.Partitions() {

		log.Printf("partition %d:", i)
		// Check mana

		resp, err := partition.Peers()[0].GoShimmerAPI.GetAllMana()
		require.NoError(t, err)
		for _, nodeStr := range resp.Consensus {
			log.Printf("NodeID %s cMana %f", nodeStr.ShortNodeID, nodeStr.Mana)
		}

		tests.CheckTransactions(t, partition.Peers(), map[string]*tests.ExpectedTransaction{
			conflictingTxIDs[i]: nil,
		}, true, tests.ExpectedInclusionState{
			Confirmed:   tests.False(),
			Finalized:   tests.False(),
			Conflicting: tests.False(),
			Solid:       tests.True(),
			Rejected:    tests.False(),
			Liked:       tests.True(),
		})
	}

	// merge back the partitions
	log.Println("merging partitions...")
	assert.NoError(t, n.DeletePartitions(), "merging the network partitions should work")
	log.Println("waiting for resolved partitions to autopeer to each other")
	err = n.WaitForAutopeering(4)
	require.NoError(t, err)

	for _, conflict := range conflictingTxs {
		// issue the reattachment on the first peer
		issuerPeer := n.Peers()[0]
		_, err := issuerPeer.SendPayload(conflict.Bytes())
		log.Printf("issued reattachment conflict transaction %s on peer %s", conflict.ID(), issuerPeer.ID().String())
		assert.NoError(t, err)
	}

	log.Println("waiting for transactions to be available on all peers...")
	missing, err := tests.AwaitTransactionAvailability(n.Peers(), conflictingTxIDs, 30*time.Second)
	if err != nil {
		assert.NoError(t, err, "transactions should have been available")
		for p, missingOnPeer := range missing {
			log.Printf("missing on peer %s:", p)
			for missingTx := range missingOnPeer {
				log.Println("tx id: ", missingTx)
			}
		}
		return
	}

	expectations := map[string]*tests.ExpectedTransaction{}
	for _, conflictingTx := range conflictingTxs {
		utilsTx := value.ParseTransaction(conflictingTx)
		expectations[conflictingTx.ID().Base58()] = &tests.ExpectedTransaction{
			Inputs:       &utilsTx.Inputs,
			Outputs:      &utilsTx.Outputs,
			UnlockBlocks: &utilsTx.UnlockBlocks,
		}
	}

	// check that the transactions are marked as conflicting
	tests.CheckTransactions(t, n.Peers(), expectations, true, tests.ExpectedInclusionState{
		Finalized:   tests.False(),
		Conflicting: tests.True(),
		Solid:       tests.True(),
	})

	// wait until the voting has finalized
	log.Println("waiting for voting/transaction finalization to be done on all peers...")
	awaitFinalization := map[string]tests.ExpectedInclusionState{}
	for _, conflictingTx := range conflictingTxs {
		awaitFinalization[conflictingTx.ID().Base58()] = tests.ExpectedInclusionState{
			Finalized: tests.True(),
		}
	}
	err = tests.AwaitTransactionInclusionState(n.Peers(), awaitFinalization, 2*time.Minute)
	assert.NoError(t, err)

	// now all transactions must be finalized and at most one must be confirmed
	var confirmedOverConflictSet int
	for _, conflictingTx := range conflictingTxIDs {
		var rejected, confirmed int
		for _, p := range n.Peers() {
			tx, err := p.GetTransactionByID(conflictingTx)
			assert.NoError(t, err)
			if tx.InclusionState.Confirmed {
				confirmed++
				continue
			}
			if tx.InclusionState.Rejected {
				rejected++
			}
		}

		if rejected != 0 {
			assert.Len(t, n.Peers(), rejected, "the rejected count for %s should be equal to the amount of peers", conflictingTx)
		}
		if confirmed != 0 {
			assert.Len(t, n.Peers(), confirmed, "the confirmed count for %s should be equal to the amount of peers", conflictingTx)
			confirmedOverConflictSet++
		}

		assert.False(t, rejected == 0 && confirmed == 0, "a transaction must either be rejected or confirmed")
	}

	// there must only be one confirmed transaction out of the conflict set
	if confirmedOverConflictSet != 0 {
		assert.Equal(t, 1, confirmedOverConflictSet, "only one transaction can be confirmed out of the conflict set. %d of %d are confirmed", confirmedOverConflictSet, len(conflictingTxIDs))
	}
}

func CreateOutputs(input *ledgerstate.UTXOInput, inputBalance int, inputSeed *seed.Seed, nOutputs int, pledgeID identity.ID) (*ledgerstate.Transaction, *seed.Seed) {
	partitionReceiverSeed := seed.NewSeed()

	destAddr := make([]ledgerstate.Address, nOutputs)
	sigLockedColoredOutputs := make([]*ledgerstate.SigLockedColoredOutput, nOutputs)
	outputs := make([]ledgerstate.Output, nOutputs)

	outputBalances := make([]uint64, nOutputs)
	// make sure the output balances are equal input
	var totalBalance uint64 = 0
	for i:=0; i < nOutputs - 1; i++ {
		outputBalances[i] = uint64(inputBalance / nOutputs)
		totalBalance, _ = ledgerstate.SafeAddUint64(totalBalance, outputBalances[i])
	}
	outputBalances[nOutputs-1], _ = ledgerstate.SafeSubUint64(uint64(inputBalance), totalBalance)
	log.Printf("Transaction balances; input: %d, output: %v", inputBalance, outputBalances)
	for i := 0; i < nOutputs; i++ {
		destAddr[i] = partitionReceiverSeed.Address(uint64(i)).Address()
		sigLockedColoredOutputs[i] = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: outputBalances[i],
		}), destAddr[i])
		outputs[i] = sigLockedColoredOutputs[i]
	}

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), pledgeID, pledgeID, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(outputs...))
	kp := inputSeed.KeyPair(0)
	sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
	return tx, partitionReceiverSeed
}

func issueTransaction(issuerPeer *framework.Peer, tx *ledgerstate.Transaction, t *testing.T, txDescription string) string {
	txID, err := issuerPeer.SendTransaction(tx.Bytes())
	assert.NoError(t, err)
	log.Printf("issued %s transaction %s on partition %d on peer %s", txDescription, txID, 0, issuerPeer.ID().String())
	return txID
}
