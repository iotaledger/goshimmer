package consensus

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestConsensusFiftyFiftyOpinionSplit spawns two network partitions with their own peers,
// then issues valid value objects spending the genesis in both, deletes the partitions (and lets them merge)
// and then checks that the conflicts are resolved via FPC.
func TestConsensusFiftyFiftyOpinionSplit(t *testing.T) {
	// override avg. network delay to accustom integration test slowness
	backupFCoBAvgNetworkDelay := framework.ParaFCoBAverageNetworkDelay
	// adjust l according to networkDelay l = l+c/roundTimeInterval
	// backupFPCTotalRoundsFinalization := framework.ParaFPCTotalRoundsFinalization
	backupBootstrapOnEveryNode := framework.ParaSyncBeaconOnEveryNode
	backupParaWaitToKill := framework.ParaWaitToKill
	framework.ParaFCoBAverageNetworkDelay = 60
	framework.ParaSyncBeaconOnEveryNode = true
	framework.ParaWaitToKill = 2*framework.ParaFCoBAverageNetworkDelay + 10
	// framework.ParaFPCTotalRoundsFinalization = backupFPCTotalRoundsFinalization + framework.ParaFCoBAverageNetworkDelay/int(framework.ParaFPCRoundInterval)

	const numberOfPeers = 6

	// reset framework paras
	defer func() {
		framework.ParaFCoBAverageNetworkDelay = backupFCoBAvgNetworkDelay
		framework.ParaSyncBeaconOnEveryNode = backupBootstrapOnEveryNode
		framework.ParaWaitToKill = backupParaWaitToKill
		// framework.ParaFPCTotalRoundsFinalization = backupFPCTotalRoundsFinalization
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

	snapshot := tests.GetSnapshot()

	faucetPledge := "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP"
	pubKey, err := ed25519.PublicKeyFromString(faucetPledge)
	if err != nil {
		panic(err)
	}
	nodeID := identity.NewID(pubKey)

	genesisTransactionID := ledgerstate.GenesisTransactionID
	for ID, tx := range snapshot.Transactions {
		if tx.AccessPledgeID() == nodeID {
			genesisTransactionID = ID
		}
	}

	// make genesis fund easily divisible for further splitting of the funds
	const genesisBalance = 1000000000000000
	genesisSeed := seed.NewSeed(genesisSeedBytes)
	genesisOutputID := ledgerstate.NewOutputID(genesisTransactionID, 0)
	input := ledgerstate.NewUTXOInput(genesisOutputID)
	// splitting genesis funds to one address per peer plus one additional that will be used for the conflict
	spendingGenTx, destGenSeed := CreateOutputs(input, genesisBalance, genesisSeed.KeyPair(0), numberOfPeers+1, identity.ID{}, "skewed")

	// issue the transaction on the first peer of each partition, both partitions will have the same view
	issueTransaction(n.Partitions()[0].Peers()[0], spendingGenTx, t, "genesis splitting", 0)
	issueTransaction(n.Partitions()[1].Peers()[0], spendingGenTx, t, "genesis splitting", 1)

	// sleep the avg. network delay so both partitions confirm their own first seen transaction
	log.Printf("waiting %d seconds avg. network delay to make the transactions "+
		"preferred in their corresponding partition", framework.ParaFCoBAverageNetworkDelay)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay) * time.Second)

	// issue one transaction per peer to pledge mana to nodes
	// leave one unspent output from splitting genesis transaction for further conflict creation

	// prepare all the pledgingTxs
	pledgingTxs := make([]*ledgerstate.Transaction, numberOfPeers+1)
	pledgeSeed := make([]*seed.Seed, numberOfPeers+1)
	receiverId := 0
	for _, partition := range n.Partitions() {
		for _, peer := range partition.Peers() {
			// get next dest addresses
			destAddr := destGenSeed.Address(uint64(receiverId)).Address()
			fmt.Printf("dest addr: %s\n", destAddr)
			// Get tx output for current dest address

			outputGenTx := spendingGenTx.Essence().Outputs().Filter(func(output ledgerstate.Output) bool {
				return output.Address().Base58() == destAddr.Base58()
			})[0]
			balance, _ := outputGenTx.Balances().Get(ledgerstate.ColorIOTA)
			pledgeInput := ledgerstate.NewUTXOInput(outputGenTx.ID())
			pledgingTxs[receiverId], pledgeSeed[receiverId] = CreateOutputs(pledgeInput, balance, destGenSeed.KeyPair(uint64(receiverId)), 1, peer.ID(), "equal")

			// issue the transaction to peers on both partitions
			issueTransaction(n.Partitions()[0].Peers()[0], pledgingTxs[receiverId], t, "pledging", 0)
			issueTransaction(n.Partitions()[1].Peers()[0], pledgingTxs[receiverId], t, "pledging", 1)
			receiverId++
		}
	}
	// sleep 3* the avg. network delay so both partitions confirm their own pledging transaction
	// and 1 avg delay more to make sure each node has mana
	log.Printf("waiting 2 * %d seconds avg. network delay + 5s to make the transactions confirmed", framework.ParaFCoBAverageNetworkDelay)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay)*2*time.Second + 5*time.Second)

	// prepare two conflicting transactions from one additional unused genesis output
	conflictingTxs := make([]*ledgerstate.Transaction, len(n.Partitions()))
	conflictingTxIDs := make([]string, len(n.Partitions()))
	receiverSeeds := make([]*seed.Seed, len(n.Partitions()))
	// get address for last created unused genesis output
	lastAddress := destGenSeed.Address(uint64(numberOfPeers)).Address()
	// Get ast created unused genesis output and its balance
	lastOutputTx := spendingGenTx.Essence().Outputs().Filter(func(output ledgerstate.Output) bool {
		return output.Address().Base58() == lastAddress.Base58()
	})[0]
	lastOutputBalance, _ := lastOutputTx.Balances().Get(ledgerstate.ColorIOTA)
	// prepare two conflicting transactions, one per partition
	for i, partition := range n.Partitions() {
		conflictInput := ledgerstate.NewUTXOInput(lastOutputTx.ID())
		conflictingTxs[i], receiverSeeds[i] = CreateOutputs(conflictInput, lastOutputBalance, destGenSeed.KeyPair(numberOfPeers), 1, partition.Peers()[0].ID(), "equal")

		// issue conflicting transaction on the current partition
		txId := issueTransaction(partition.Peers()[0], conflictingTxs[i], t, "conflicting", 0)
		conflictingTxIDs[i] = txId
	}

	// sleep the avg. network delay so both partitions prefer their own first seen transaction
	log.Printf("waiting %d seconds, 1/4th of avg. network delay before merging the partitions", framework.ParaFCoBAverageNetworkDelay/4)
	time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay) / 4 * time.Second)
	premergeTimestamp := time.Now()
	// merge back the partitions
	log.Println("merging partitions...")
	assert.NoError(t, n.DeletePartitions(), "merging the network partitions should work")
	log.Println("waiting for resolved partitions to autopeer to each other")
	err = n.WaitForAutopeering(4)
	require.NoError(t, err)

	diff := time.Since(premergeTimestamp)
	if diff < time.Duration(framework.ParaFCoBAverageNetworkDelay)*time.Second {
		log.Printf("waiting remaining  %d seconds of avg. network delay to make the transactions "+
			"preferred in their corresponding partition", framework.ParaFCoBAverageNetworkDelay-int(diff.Seconds()))
		time.Sleep(time.Duration(framework.ParaFCoBAverageNetworkDelay)*time.Second - diff)
	}

	for _, conflict := range conflictingTxs {
		// issue the reattachment on the first peer
		issuerPeer := n.Peers()[0]
		_, err := issuerPeer.SendPayload(conflict.Bytes())
		log.Printf("issued reattachment conflict transaction %s on peer %s", conflict.ID(), issuerPeer.ID().String())
		assert.NoError(t, err)
	}

	log.Println("waiting for transactions to be available on all peers...")
	missing, err := tests.AwaitTransactionAvailability(n.Peers(), conflictingTxIDs, time.Duration(framework.ParaFCoBAverageNetworkDelay)*time.Second)
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

	err = tests.AwaitTransactionInclusionState(n.Peers(), awaitFinalization, 30*time.Duration(framework.ParaFPCRoundInterval)*time.Second)
	assert.NoError(t, err)

	// now all transactions must be finalized and at most one must be confirmed
	rejected := make([]int, 2)
	confirmed := make([]int, 2)

	for i, conflictingTx := range conflictingTxIDs {
		for _, p := range n.Peers() {
			tx, err := p.GetTransactionByID(conflictingTx)
			assert.NoError(t, err)
			if tx.InclusionState.Confirmed {
				confirmed[i]++
				continue
			}
			if tx.InclusionState.Rejected {
				rejected[i]++
			}
		}
	}

	assert.Equal(t, 0, rejected[0], "the rejected count for first transaction should be equal to 0")
	assert.Equal(t, len(n.Peers()), rejected[1], "the rejected count for second transaction should be equal to %d", len(n.Peers()))
	assert.Equal(t, 0, confirmed[1], "the confirmed count for second transaction should be equal to 0")
	assert.Equal(t, len(n.Peers()), confirmed[0], "the confirmed count for first transaction should be equal to the amount of peers %d", len(n.Peers()))
}

func CreateOutputs(input *ledgerstate.UTXOInput, inputBalance uint64, kp *ed25519.KeyPair, nOutputs int, pledgeID identity.ID, balanceType string) (*ledgerstate.Transaction, *seed.Seed) {
	partitionReceiverSeed := seed.NewSeed()

	destAddr := make([]ledgerstate.Address, nOutputs)
	sigLockedColoredOutputs := make([]*ledgerstate.SigLockedColoredOutput, nOutputs)
	outputs := make([]ledgerstate.Output, nOutputs)
	outputBalances := createBalances(balanceType, nOutputs, inputBalance)
	for i := 0; i < nOutputs; i++ {
		destAddr[i] = partitionReceiverSeed.Address(uint64(i)).Address()
		sigLockedColoredOutputs[i] = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: outputBalances[i],
		}), destAddr[i])
		outputs[i] = sigLockedColoredOutputs[i]
	}

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), pledgeID, pledgeID, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(outputs...))

	sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	tx := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})
	return tx, partitionReceiverSeed
}

func issueTransaction(issuerPeer *framework.Peer, tx *ledgerstate.Transaction, t *testing.T, txDescription string, partNum int) string {
	txID, err := issuerPeer.SendTransaction(tx.Bytes())
	assert.NoError(t, err)
	log.Printf("issued %s transaction %s on partition %d on peer %s", txDescription, txID, partNum, issuerPeer.ID().String())
	return txID
}

func createBalances(balanceType string, nOutputs int, inputBalance uint64) []uint64 {
	outputBalances := make([]uint64, 0)
	// make sure the output balances are equal input
	var totalBalance uint64 = 0
	switch balanceType {
	// input is divided equally among outputs
	case "equal":
		for i := 0; i < nOutputs-1; i++ {
			outputBalances = append(outputBalances, inputBalance/uint64(nOutputs))
			totalBalance, _ = ledgerstate.SafeAddUint64(totalBalance, outputBalances[i])
		}
		lastBalance, _ := ledgerstate.SafeSubUint64(inputBalance, totalBalance)
		outputBalances = append(outputBalances, lastBalance)
		fmt.Printf("equal balances %v", outputBalances)
	// first output gets 90% of all input funds
	case "skewed":
		if nOutputs == 1 {
			outputBalances = append(outputBalances, inputBalance)
			fmt.Println("one balance")
		} else {
			fmt.Printf("before %v", outputBalances)
			outputBalances = append(outputBalances, inputBalance*9/10)
			remainingBalance, _ := ledgerstate.SafeSubUint64(inputBalance, outputBalances[0])
			fmt.Printf("remainig %v", outputBalances)
			for i := 1; i < nOutputs-1; i++ {
				outputBalances = append(outputBalances, remainingBalance/uint64(nOutputs-1))
				totalBalance, _ = ledgerstate.SafeAddUint64(totalBalance, outputBalances[i])
			}
			lastBalance, _ := ledgerstate.SafeSubUint64(remainingBalance, totalBalance)
			outputBalances = append(outputBalances, lastBalance)
			// outputBalances = append(outputBalances, createBalances("equal", nOutputs-1, remainingBalance)...)
			fmt.Printf("rady %v", outputBalances)
		}
	}
	log.Printf("Transaction balances; input: %d, output: %v", inputBalance, outputBalances)
	return outputBalances
}
