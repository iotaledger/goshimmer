package consensus

import (
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestConsensusNoConflicts issues valid non-conflicting value objects and then checks
// whether the ledger of every peer reflects the same correct state.
func TestConsensusNoConflicts(t *testing.T) {
	n, err := f.CreateNetwork("consensus_TestConsensusNoConflicts", 4, 2, framework.CreateNetworkConfig{})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	time.Sleep(5 * time.Second)

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

	const genesisBalance = 1000000000000000
	genesisSeed := seed.NewSeed(genesisSeedBytes)
	genesisAddr := genesisSeed.Address(0).Address()
	genesisOutputID := ledgerstate.NewOutputID(genesisTransactionID, 0)
	input := ledgerstate.NewUTXOInput(genesisOutputID)

	firstReceiver := seed.NewSeed()
	const depositCount = 10
	const deposit = genesisBalance / depositCount
	firstReceiverAddresses := make([]string, depositCount)
	firstReceiverDepositAddrs := make([]ledgerstate.Address, depositCount)
	firstReceiverDepositOutputs := make(map[ledgerstate.Address]*ledgerstate.ColoredBalances)
	firstReceiverExpectedBalances := make(map[string]map[ledgerstate.Color]int64)
	for i := 0; i < depositCount; i++ {
		addr := firstReceiver.Address(uint64(i)).Address()
		firstReceiverDepositAddrs[i] = addr
		firstReceiverAddresses[i] = addr.Base58()
		firstReceiverDepositOutputs[addr] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: deposit})
		firstReceiverExpectedBalances[addr.Base58()] = map[ledgerstate.Color]int64{ledgerstate.ColorIOTA: deposit}
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
	utilsTx := value.ParseTransaction(tx1)

	txID, err := n.Peers()[0].SendTransaction(tx1.Bytes())
	require.NoError(t, err)

	// wait for the transaction to be propagated through the network
	// and it becoming preferred, finalized and confirmed
	log.Println("waiting 2.5 avg. network delays")
	time.Sleep(messagelayer.DefaultAverageNetworkDelay*2 + messagelayer.DefaultAverageNetworkDelay/2)

	// since we just issued a transaction spending the genesis output, there
	// shouldn't be any UTXOs on the genesis address anymore
	log.Println("checking that genesis has no UTXOs")
	tests.CheckAddressOutputsFullyConsumed(t, n.Peers(), []string{genesisAddr.Base58()})

	// since we waited 2.5 avg. network delays and there were no conflicting transactions,
	// the transaction we just issued must be preferred, liked, finalized and confirmed
	log.Println("check that the transaction is finalized/confirmed by all peers")
	tests.CheckTransactions(t, n.Peers(), map[string]*tests.ExpectedTransaction{
		txID: {Inputs: &utilsTx.Inputs, Outputs: &utilsTx.Outputs, UnlockBlocks: &utilsTx.UnlockBlocks},
	}, true, tests.ExpectedInclusionState{
		Confirmed: tests.True(), Finalized: tests.True(),
		Conflicting: tests.False(), Solid: tests.True(),
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

		txID, err := n.Peers()[rand.Intn(len(n.Peers()))].SendTransaction(tx.Bytes())
		require.NoError(t, err)

		utilsTx := value.ParseTransaction(tx)

		secondReceiverExpectedBalances[addr.Base58()] = map[ledgerstate.Color]int64{ledgerstate.ColorIOTA: deposit}
		secondReceiverExpectedTransactions[txID] = &tests.ExpectedTransaction{
			Inputs: &utilsTx.Inputs, Outputs: &utilsTx.Outputs, UnlockBlocks: &utilsTx.UnlockBlocks,
		}
	}

	// wait again some network delays for the transactions to materialize
	log.Println("waiting 2.5 avg. network delays")
	time.Sleep(messagelayer.DefaultAverageNetworkDelay*2 + messagelayer.DefaultAverageNetworkDelay/2)
	log.Println("checking that first set of addresses contain no UTXOs")
	tests.CheckAddressOutputsFullyConsumed(t, n.Peers(), firstReceiverAddresses)
	log.Println("checking that the 2nd batch transactions are finalized/confirmed")
	tests.CheckTransactions(t, n.Peers(), secondReceiverExpectedTransactions, true,
		tests.ExpectedInclusionState{
			Confirmed: tests.True(), Finalized: tests.True(),
			Conflicting: tests.False(), Solid: tests.True(),
			Rejected: tests.False(), Liked: tests.True(),
		},
	)

	log.Println("check that the 2nd batch of receive addresses is the same on all peers")
	tests.CheckBalances(t, n.Peers(), secondReceiverExpectedBalances)
}
