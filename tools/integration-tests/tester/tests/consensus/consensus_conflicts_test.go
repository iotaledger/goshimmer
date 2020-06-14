package consensus

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsensusConflicts issues valid conflicting value objects and makes sure that
// the conflicts are resolved via FPC.
func TestConsensusConflicts(t *testing.T) {
	n, err := f.CreateNetworkWithPartitions("consensus_TestConsensusConflicts", 8, 2, 4)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	time.Sleep(10 * time.Second)

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
	genesisWallet := wallet.New(genesisSeedBytes)
	genesisAddr := genesisWallet.Seed().Address(0)
	genesisOutputID := transaction.NewOutputID(genesisAddr, transaction.GenesisID)

	// issue transactions which spend the same genesis output in all partitions
	conflictingTxs := make([]*transaction.Transaction, len(n.Partitions()))
	conflictingTxIDs := make([]string, len(n.Partitions()))
	receiverWallets := make([]*wallet.Wallet, len(n.Partitions()))
	for i, partition := range n.Partitions() {

		// create a new receiver wallet for the given partition
		partitionReceiverWallet := wallet.New()
		destAddr := partitionReceiverWallet.Seed().Address(0)
		receiverWallets[i] = partitionReceiverWallet
		tx := transaction.New(
			transaction.NewInputs(genesisOutputID),
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				destAddr: {
					{Value: genesisBalance / 2, Color: balance.ColorIOTA},
				},
			}))
		tx = tx.Sign(signaturescheme.ED25519(*genesisWallet.Seed().KeyPair(0)))
		conflictingTxs[i] = tx

		// issue the transaction on the first peer of the partition
		issuerPeer := partition.Peers()[0]
		txID, err := issuerPeer.SendTransaction(tx.Bytes())
		conflictingTxIDs[i] = txID
		log.Printf("issued conflict transaction %s on partition %d on peer %s", txID, i, issuerPeer.ID().String())
		assert.NoError(t, err)

		// check that the transaction is actually available on all the peers of the partition
		missing, err := tests.AwaitTransactionAvailability(partition.Peers(), []string{txID}, 4*time.Second)
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
	time.Sleep(valuetransfers.AverageNetworkDelay)

	// merge back the partitions
	log.Println("merging partitions...")
	assert.NoError(t, n.DeletePartitions(), "merging the network partitions should work")
	log.Println("waiting for resolved partitions to autopeer to each other")
	err = n.WaitForAutopeering(5)
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
		utilsTx := utils.ParseTransaction(conflictingTx)
		expectations[conflictingTx.ID().String()] = &tests.ExpectedTransaction{
			Inputs:    &utilsTx.Inputs,
			Outputs:   &utilsTx.Outputs,
			Signature: &utilsTx.Signature,
		}
	}

	tests.CheckTransactions(t, n.Peers(), expectations, true, tests.ExpectedInclusionState{
		Confirmed: tests.False(),
		Finalized: tests.False(),
		// should be part of a conflict set
		Conflict: tests.True(),
		Solid:    tests.True(),
		Rejected: tests.False(),
		Liked:    tests.False(),
	})
}
