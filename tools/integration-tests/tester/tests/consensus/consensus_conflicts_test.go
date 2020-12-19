package consensus

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"

	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	valueutils "github.com/iotaledger/goshimmer/plugins/webapi/value"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
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
	framework.ParaFCoBAverageNetworkDelay = 30
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

	const genesisBalance = 1000000000
	genesisSeed := walletseed.NewSeed(genesisSeedBytes)
	genesisAddr := genesisSeed.Address(0).Address
	genesisOutputID := transaction.NewOutputID(genesisAddr, transaction.GenesisID)

	// issue transactions which spend the same genesis output in all partitions
	conflictingTxs := make([]*transaction.Transaction, len(n.Partitions()))
	conflictingTxIDs := make([]string, len(n.Partitions()))
	receiverSeeds := make([]*walletseed.Seed, len(n.Partitions()))
	for i, partition := range n.Partitions() {

		// create a new receiver wallet for the given partition
		partitionReceiverSeed := walletseed.NewSeed()
		destAddr := partitionReceiverSeed.Address(0).Address
		receiverSeeds[i] = partitionReceiverSeed
		tx := transaction.New(
			transaction.NewInputs(genesisOutputID),
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				destAddr: {
					{Value: genesisBalance, Color: balance.ColorIOTA},
				},
			}))
		tx = tx.Sign(signaturescheme.ED25519(*genesisSeed.KeyPair(0)))
		conflictingTxs[i] = tx

		// issue the transaction on the first peer of the partition
		issuerPeer := partition.Peers()[0]
		txID, err := issuerPeer.SendTransaction(tx.Bytes())
		conflictingTxIDs[i] = txID
		log.Printf("issued conflict transaction %s on partition %d on peer %s", txID, i, issuerPeer.ID().String())
		assert.NoError(t, err)

		// check that the transaction is actually available on all the peers of the partition
		missing, err := tests.AwaitTransactionAvailability(partition.Peers(), []string{txID}, 15*time.Second)
		if err != nil {
			assert.NoError(t, err, "transactions should have been available in partition")
			for p, missingOnPeer := range missing {
				log.Printf("missing on peer %s:", p)
				for missingTx := range missingOnPeer {
					log.Println("tx id: ", missingTx)
				}
			}
			return
		}

		require.NoError(t, err)
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
			Preferred:   tests.True(),
		})
	}

	// merge back the partitions
	log.Println("merging partitions...")
	assert.NoError(t, n.DeletePartitions(), "merging the network partitions should work")
	log.Println("waiting for resolved partitions to autopeer to each other")
	err = n.WaitForAutopeering(4)
	require.NoError(t, err)

	// ensure message flow so that both partitions will get the conflicting tx
	for _, p := range n.Peers() {
		tests.SendDataMessage(t, p, []byte("DATA"), 10)
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
		utilsTx := valueutils.ParseTransaction(conflictingTx)
		expectations[conflictingTx.ID().String()] = &tests.ExpectedTransaction{
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
		awaitFinalization[conflictingTx.ID().String()] = tests.ExpectedInclusionState{
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
