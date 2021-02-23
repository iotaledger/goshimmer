package consensus

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsensusFiftyFiftyOpinionSplit spawns two network partitions with their own peers,
// then issues valid value objects spending the genesis in both, deletes the partitions (and lets them merge)
// and then checks that the conflicts are resolved via FPC.
func TestConsensusFiftyFiftyOpinionSplit(t *testing.T) {

	// override avg. network delay to accustom integration test slowness
	backupFCoBAvgNetworkDelay := framework.ParaFCoBAverageNetworkDelay
	backupBootstrapOnEveryNode := framework.ParaSyncBeaconOnEveryNode
	backupParaWaitToKill := framework.ParaWaitToKill
	framework.ParaFCoBAverageNetworkDelay = 60
	framework.ParaSyncBeaconOnEveryNode = true
	framework.ParaWaitToKill = 2*framework.ParaFCoBAverageNetworkDelay + 10

	// reset framework paras
	defer func() {
		framework.ParaFCoBAverageNetworkDelay = backupFCoBAvgNetworkDelay
		framework.ParaSyncBeaconOnEveryNode = backupBootstrapOnEveryNode
		framework.ParaWaitToKill = backupParaWaitToKill
	}()

	// create two partitions with their own peers
	n, err := f.CreateNetworkWithPartitions("abc", 6, 2, 2)
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

	const genesisBalance = 1000000000000000
	genesisSeed := seed.NewSeed(genesisSeedBytes)
	genesisOutputID := ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0)
	input := ledgerstate.NewUTXOInput(genesisOutputID)

	// issue transactions which spend the same genesis output in all partitions
	conflictingTxs := make([]*ledgerstate.Transaction, len(n.Partitions()))
	conflictingTxIDs := make([]string, len(n.Partitions()))
	receiverSeeds := make([]*seed.Seed, len(n.Partitions()))

	for i, partition := range n.Partitions() {

		// create a new receiver wallet for the given partition
		partitionReceiverSeed := seed.NewSeed()
		destAddr := partitionReceiverSeed.Address(0).Address()
		receiverSeeds[i] = partitionReceiverSeed
		output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: uint64(genesisBalance),
		}), destAddr)
		txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output))
		kp := genesisSeed.KeyPair(0)
		sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
		unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
		tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
		conflictingTxs[i] = tx

		// issue the transaction on the first peer of the partition
		issuerPeer := partition.Peers()[0]
		txID, err := issuerPeer.SendTransaction(tx.Bytes())
		conflictingTxIDs[i] = txID
		log.Printf("issued conflict transaction %s on partition %d on peer %s", txID, i, issuerPeer.ID().String())
		assert.NoError(t, err)
	}

	// sleep the avg. network delay so both partitions prefer their own first seen transaction
	log.Printf("waiting %d seconds avg. network delay to make the transactions "+
		"preferred in their corresponding partition", framework.ParaFCoBAverageNetworkDelay)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay) * time.Second)

	// check that each partition is preferring its corresponding transaction
	log.Println("checking that each partition likes its corresponding transaction before the conflict:")
	for i, partition := range n.Partitions() {
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
			Inputs:    &utilsTx.Inputs,
			Outputs:   &utilsTx.Outputs,
			Signature: &utilsTx.Signature,
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
