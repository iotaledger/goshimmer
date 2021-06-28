package consensus

import (
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestConsensusNoConflicts issues valid non-conflicting value objects and then checks
// whether the ledger of every peer reflects the same correct state.
func TestConsensusNoConflicts(t *testing.T) {
	n, err := f.CreateNetwork("consensus_TestConsensusNoConflicts", 4, framework.CreateNetworkConfig{Faucet: true, StartSynced: true})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	time.Sleep(5 * time.Second)

	// genesis wallet
	genesisSeedBytes, err := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
	require.NoError(t, err, "couldn't decode genesis seed from base58 seed")

	genesisSeed := seed.NewSeed(genesisSeedBytes)
	genesisAddr := genesisSeed.Address(0).Address()
	unspentOutputs, err := n.Peers()[0].PostAddressUnspentOutputs([]string{genesisAddr.Base58()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", n.Peers()[0].String())
	genesisOutput, err := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	genesisBalance, exist := genesisOutput.Balances().Get(ledgerstate.ColorIOTA)
	assert.True(t, exist)
	input := ledgerstate.NewUTXOInput(genesisOutput.ID())

	firstReceiver := seed.NewSeed()
	const depositCount = 10
	deposit := uint64(genesisBalance / depositCount)
	firstReceiverAddresses := make([]string, depositCount)
	firstReceiverDepositAddrs := make([]ledgerstate.Address, depositCount)
	firstReceiverDepositOutputs := make(map[ledgerstate.Address]*ledgerstate.ColoredBalances)
	firstReceiverExpectedBalances := make(map[string]map[ledgerstate.Color]int64)
	for i := 0; i < depositCount; i++ {
		addr := firstReceiver.Address(uint64(i)).Address()
		firstReceiverDepositAddrs[i] = addr
		firstReceiverAddresses[i] = addr.Base58()
		firstReceiverDepositOutputs[addr] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: deposit})
		firstReceiverExpectedBalances[addr.Base58()] = map[ledgerstate.Color]int64{ledgerstate.ColorIOTA: int64(deposit)}
	}

	// issue transaction spending from the genesis output
	var outputs ledgerstate.Outputs
	for addr := range firstReceiverDepositOutputs {
		outputs = append(outputs, ledgerstate.NewSigLockedSingleOutput(deposit, addr))
	}
	log.Printf("issuing transaction spending genesis to %d addresses", depositCount)
	tx1Essence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(outputs...))
	kp := *genesisSeed.KeyPair(0)
	sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(tx1Essence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	tx1 := ledgerstate.NewTransaction(tx1Essence, ledgerstate.UnlockBlocks{unlockBlock})
	utilsTx := jsonmodels.NewTransaction(tx1)

	resp, err := n.Peers()[0].PostTransaction(tx1.Bytes())
	require.NoError(t, err)
	txID := resp.TransactionID

	// wait for the transaction to be propagated through the network
	// and it becoming preferred, finalized and confirmed
	log.Println("waiting 2.5 avg. network delays")
	time.Sleep(framework.DefaultUpperBoundNetworkDelay*2 + framework.DefaultUpperBoundNetworkDelay/2)

	// since we just issued a transaction spending the genesis output, there
	// shouldn't be any UTXOs on the genesis address anymore
	log.Println("checking that genesis has no UTXOs")
	tests.CheckAddressOutputsFullyConsumed(t, n.Peers(), []string{genesisAddr.Base58()})

	// Wait for the approval weigth to build up via the sync beacon.
	time.Sleep(20 * time.Second)
	log.Println("check that the transaction is finalized/confirmed by all peers")
	tests.CheckTransactions(t, n.Peers(), map[string]*tests.ExpectedTransaction{
		txID: {Inputs: utilsTx.Inputs, Outputs: utilsTx.Outputs, UnlockBlocks: utilsTx.UnlockBlocks},
	}, true, tests.ExpectedInclusionState{
		Confirmed: tests.True(), Finalized: tests.True(),
		Conflicting: tests.False(), SolidityType: tests.Solid(),
		Rejected: tests.False(), Liked: tests.True(),
	})

	// check balances on peers
	log.Println("ensure that all the peers have the same ledger state")
	tests.CheckBalances(t, n.Peers(), firstReceiverExpectedBalances)

	// issue transactions spending all the outputs which were just created from a random peer
	secondReceiverSeed := seed.NewSeed()
	secondReceiverAddresses := make([]string, depositCount)
	secondReceiverExpectedBalances := map[string]map[ledgerstate.Color]int64{}
	secondReceiverExpectedTransactions := map[string]*tests.ExpectedTransaction{}

	for i := 0; i < depositCount; i++ {
		addr := secondReceiverSeed.Address(uint64(i)).Address()
		input := ledgerstate.NewUTXOInput(tx1.Essence().Outputs()[int(tests.SelectIndex(tx1, firstReceiver.Address(uint64(i)).Address()))].ID())
		output := ledgerstate.NewSigLockedSingleOutput(deposit, addr)
		tx2Essence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output))
		kp := *firstReceiver.KeyPair(uint64(i))
		sig := ledgerstate.NewED25519Signature(kp.PublicKey, kp.PrivateKey.Sign(tx2Essence.Bytes()))
		unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
		tx := ledgerstate.NewTransaction(tx2Essence, ledgerstate.UnlockBlocks{unlockBlock})
		secondReceiverAddresses[i] = addr.Base58()

		resp, err := n.Peers()[rand.Intn(len(n.Peers()))].PostTransaction(tx.Bytes())
		require.NoError(t, err)
		txID := resp.TransactionID

		utilsTx := jsonmodels.NewTransaction(tx)

		secondReceiverExpectedBalances[addr.Base58()] = map[ledgerstate.Color]int64{ledgerstate.ColorIOTA: int64(deposit)}
		secondReceiverExpectedTransactions[txID] = &tests.ExpectedTransaction{
			Inputs: utilsTx.Inputs, Outputs: utilsTx.Outputs, UnlockBlocks: utilsTx.UnlockBlocks,
		}
	}

	// wait again some network delays for the transactions to materialize
	log.Println("waiting 2.5 avg. network delays")
	time.Sleep(framework.DefaultUpperBoundNetworkDelay*2 + framework.DefaultUpperBoundNetworkDelay/2)
	log.Println("checking that first set of addresses contain no UTXOs")
	tests.CheckAddressOutputsFullyConsumed(t, n.Peers(), firstReceiverAddresses)

	// Wait for the approval weigth to build up via the sync beacon.
	time.Sleep(20 * time.Second)
	log.Println("checking that the 2nd batch transactions are finalized/confirmed")
	tests.CheckTransactions(t, n.Peers(), secondReceiverExpectedTransactions, true,
		tests.ExpectedInclusionState{
			Confirmed: tests.True(), Finalized: tests.True(),
			Conflicting: tests.False(), SolidityType: tests.Solid(),
			Rejected: tests.False(), Liked: tests.True(),
		},
	)

	log.Println("check that the 2nd batch of receive addresses is the same on all peers")
	tests.CheckBalances(t, n.Peers(), secondReceiverExpectedBalances)
}
