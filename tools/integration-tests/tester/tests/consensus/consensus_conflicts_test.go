package consensus

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

func TestConsensusConflicts(t *testing.T) {
	const numberOfPeers = 6
	FCoBQuarantineTime := time.Duration(framework.PeerConfig().FCOB.QuarantineTime) * time.Second

	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), numberOfPeers, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Autopeering: true,
		Activity:    true,
		FPC:         true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet := n.Peers()[0]

	genesisSeed := seed.NewSeed(framework.GenesisSeed)
	genesisAddr := genesisSeed.Address(0).Address()
	unspentOutputs, err := faucet.PostAddressUnspentOutputs([]string{genesisAddr.Base58()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", n.Peers()[0].String())
	genesisOutput, err := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	genesisBalance, exist := genesisOutput.Balances().Get(ledgerstate.ColorIOTA)
	assert.True(t, exist)
	input := ledgerstate.NewUTXOInput(genesisOutput.ID())

	// splitting genesis funds to one address per peer plus one additional that will be used for the conflict
	// split funds to different addresses of destGenSeed
	spendingGenTx, destGenSeed := CreateOutputs(input, genesisBalance, genesisSeed.KeyPair(0), numberOfPeers+1, identity.ID{}, "skewed")

	// issue the transaction
	_, err = n.Peers()[0].PostTransaction(spendingGenTx.Bytes())
	assert.NoError(t, err)

	// sleep the avg. network delay so both partitions confirm their own first seen transaction
	log.Printf("waiting %v avg. network delay to make the transactions "+
		"preferred in their corresponding partition", FCoBQuarantineTime)
	time.Sleep(FCoBQuarantineTime)

	// issue one transaction per peer to pledge mana to nodes
	// leave one unspent output from splitting genesis transaction for further conflict creation

	// prepare all the pledgingTxs
	pledgingTxs := make([]*ledgerstate.Transaction, numberOfPeers+1)
	pledgeSeed := make([]*seed.Seed, numberOfPeers+1)
	receiverID := 0
	for _, peer := range n.Peers() {
		// get next dest addresses
		destAddr := destGenSeed.Address(uint64(receiverID)).Address()
		fmt.Printf("dest addr: %s\n", destAddr)
		// Get tx output for current dest address

		outputGenTx := spendingGenTx.Essence().Outputs().Filter(func(output ledgerstate.Output) bool {
			return output.Address().Base58() == destAddr.Base58()
		})[0]
		balance, _ := outputGenTx.Balances().Get(ledgerstate.ColorIOTA)
		pledgeInput := ledgerstate.NewUTXOInput(outputGenTx.ID())
		pledgingTxs[receiverID], pledgeSeed[receiverID] = CreateOutputs(pledgeInput, balance, destGenSeed.KeyPair(uint64(receiverID)), 1, peer.ID(), "equal")

		// issue the transaction
		_, err = n.Peers()[0].PostTransaction(pledgingTxs[receiverID].Bytes())
		assert.NoError(t, err)
		receiverID++
		time.Sleep(2 * time.Second)
	}
	// sleep 3* the avg. network delay so both partitions confirm their own pledging transaction
	// and 1 avg delay more to make sure each node has mana
	log.Printf("waiting 2 * %v avg. network delay + 5s to make the transactions confirmed", FCoBQuarantineTime)
	time.Sleep(2*FCoBQuarantineTime + 5*time.Second)

	_, err = n.Peers()[0].GoShimmerAPI.GetAllMana()
	require.NoError(t, err)

	// prepare two conflicting transactions from one additional unused genesis output
	conflictingTxs := make([]*ledgerstate.Transaction, 2)
	conflictingTxIDs := make([]string, 2)
	receiverSeeds := make([]*seed.Seed, 2)
	// get address for last created unused genesis output
	lastAddress := destGenSeed.Address(uint64(numberOfPeers)).Address()
	// Get ast created unused genesis output and its balance
	lastOutputTx := spendingGenTx.Essence().Outputs().Filter(func(output ledgerstate.Output) bool {
		return output.Address().Base58() == lastAddress.Base58()
	})[0]
	lastOutputBalance, _ := lastOutputTx.Balances().Get(ledgerstate.ColorIOTA)
	// prepare two conflicting transactions, one per partition
	for i, peer := range n.Peers()[1:3] {
		conflictInput := ledgerstate.NewUTXOInput(lastOutputTx.ID())
		conflictingTxs[i], receiverSeeds[i] = CreateOutputs(conflictInput, lastOutputBalance, destGenSeed.KeyPair(numberOfPeers), 1, peer.ID(), "equal")

		// issue conflicting transaction
		resp, err2 := peer.PostTransaction(conflictingTxs[i].Bytes())
		require.NoError(t, err2)
		conflictingTxIDs[i] = resp.TransactionID

		// sleep to prefer the first one
		time.Sleep(FCoBQuarantineTime)
	}

	expStates := map[string]tests.ExpectedInclusionState{}
	for _, txID := range conflictingTxIDs {
		expStates[txID] = tests.ExpectedInclusionState{
			Conflicting: tests.True(),
			Solid:       tests.True(),
		}
	}
	tests.RequireInclusionStateEqual(t, n.Peers(), expStates, tests.Timeout, tests.Tick)

	expTransactions := map[string]*tests.ExpectedTransaction{}
	for _, conflictingTx := range conflictingTxs {
		utilsTx := jsonmodels.NewTransaction(conflictingTx)
		expTransactions[conflictingTx.ID().Base58()] = &tests.ExpectedTransaction{
			Inputs:       utilsTx.Inputs,
			Outputs:      utilsTx.Outputs,
			UnlockBlocks: utilsTx.UnlockBlocks,
		}
	}

	// check that the transactions are marked as conflicting
	tests.RequireTransactionsEqual(t, n.Peers(), expTransactions)

	// wait until the voting has finalized
	log.Println("Waiting for voting/transaction finalization to be done on all peers...")
	expStates[conflictingTxIDs[0]] = tests.ExpectedInclusionState{
		Finalized: tests.True(),
	}
	expStates[conflictingTxIDs[1]] = tests.ExpectedInclusionState{
		Finalized: tests.False(),
	}
	tests.RequireInclusionStateEqual(t, n.Peers(), expStates, tests.Timeout, tests.Tick)

	// now all transactions must be finalized and at most one must be confirmed
	rejected := make([]int, 2)
	confirmed := make([]int, 2)

	for i, conflictingTx := range conflictingTxIDs {
		for _, p := range n.Peers() {
			tx, err := p.GetTransactionInclusionState(conflictingTx)
			assert.NoError(t, err)
			if tx.Confirmed {
				confirmed[i]++
				continue
			}
			if tx.Rejected {
				rejected[i]++
			}
		}
	}

	assert.Equal(t, 0, rejected[0], "the rejected count for first transaction should be equal to 0")
	assert.Equal(t, len(n.Peers()), rejected[1], "the rejected count for second transaction should be equal to %d", len(n.Peers()))
	assert.Equal(t, 0, confirmed[1], "the confirmed count for second transaction should be equal to 0")
	assert.Equal(t, len(n.Peers()), confirmed[0], "the confirmed count for first transaction should be equal to the amount of peers %d", len(n.Peers()))

	log.Println("Waiting for the potentially last rounds...")
	time.Sleep(30 * time.Second)
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
			fmt.Printf("remaining %v", outputBalances)
			for i := 1; i < nOutputs-1; i++ {
				outputBalances = append(outputBalances, remainingBalance/uint64(nOutputs-1))
				totalBalance, _ = ledgerstate.SafeAddUint64(totalBalance, outputBalances[i])
			}
			lastBalance, _ := ledgerstate.SafeSubUint64(remainingBalance, totalBalance)
			outputBalances = append(outputBalances, lastBalance)
			// outputBalances = append(outputBalances, createBalances("equal", nOutputs-1, remainingBalance)...)
			fmt.Printf("ready %v", outputBalances)
		}
	}
	log.Printf("Transaction balances; input: %d, output: %v", inputBalance, outputBalances)
	return outputBalances
}
