package consensus

import (
	"log"
	"math/rand"
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
	"github.com/stretchr/testify/require"
)

// TestConsensusNoConflicts issues valid non-conflicting value objects and then checks
// whether the ledger of every peer reflects the same correct state.
func TestConsensusNoConflicts(t *testing.T) {
	n, err := f.CreateNetwork("consensus_TestConsensusNoConflicts", 4, 2)
	require.NoError(t, err)
	defer tests.ShutdownNetwork(t, n)

	time.Sleep(5 * time.Second)

	// genesis wallet
	genesisSeedBytes, err := base58.Decode("7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih")
	require.NoError(t, err, "couldn't decode genesis seed from base58 seed")

	const genesisBalance = 1000000000
	genesisWallet := wallet.New(genesisSeedBytes)
	genesisAddr := genesisWallet.Seed().Address(0)
	genesisOutputID := transaction.NewOutputID(genesisAddr, transaction.GenesisID)

	firstReceiver := wallet.New()
	const depositCount = 10
	const deposit = genesisBalance / depositCount
	firstReceiverAddresses := make([]string, depositCount)
	firstReceiverDepositAddrs := make([]address.Address, depositCount)
	firstReceiverDepositOutputs := map[address.Address][]*balance.Balance{}
	firstReceiverExpectedBalances := map[string]map[balance.Color]int64{}
	for i := 0; i < depositCount; i++ {
		addr := firstReceiver.Seed().Address(uint64(i))
		firstReceiverDepositAddrs[i] = addr
		firstReceiverAddresses[i] = addr.String()
		firstReceiverDepositOutputs[addr] = []*balance.Balance{{Value: deposit, Color: balance.ColorIOTA}}
		firstReceiverExpectedBalances[addr.String()] = map[balance.Color]int64{balance.ColorIOTA: deposit}
	}

	// issue transaction spending from the genesis output
	log.Printf("issuing transaction spending genesis to %d addresses", depositCount)
	tx := transaction.New(transaction.NewInputs(genesisOutputID), transaction.NewOutputs(firstReceiverDepositOutputs))
	tx = tx.Sign(signaturescheme.ED25519(*genesisWallet.Seed().KeyPair(0)))
	utilsTx := utils.ParseTransaction(tx)

	txID, err := n.Peers()[0].SendTransaction(tx.Bytes())
	require.NoError(t, err)

	// wait for the transaction to be propagated through the network
	// and it becoming preferred, finalized and confirmed
	log.Println("waiting 2.5 avg. network delays")
	time.Sleep(valuetransfers.DefaultAverageNetworkDelay*2 + valuetransfers.DefaultAverageNetworkDelay/2)

	// since we just issued a transaction spending the genesis output, there
	// shouldn't be any UTXOs on the genesis address anymore
	log.Println("checking that genesis has no UTXOs")
	tests.CheckAddressOutputsFullyConsumed(t, n.Peers(), []string{genesisAddr.String()})

	// since we waited 2.5 avg. network delays and there were no conflicting transactions,
	// the transaction we just issued must be preferred, liked, finalized and confirmed
	log.Println("check that the transaction is finalized/confirmed by all peers")
	tests.CheckTransactions(t, n.Peers(), map[string]*tests.ExpectedTransaction{
		txID: {Inputs: &utilsTx.Inputs, Outputs: &utilsTx.Outputs, Signature: &utilsTx.Signature},
	}, true, tests.ExpectedInclusionState{
		Confirmed: tests.True(), Finalized: tests.True(),
		Conflicting: tests.False(), Solid: tests.True(),
		Rejected: tests.False(), Liked: tests.True(),
	})

	// check balances on peers
	log.Println("ensure that all the peers have the same ledger state")
	tests.CheckBalances(t, n.Peers(), firstReceiverExpectedBalances)

	// issue transactions spending all the outputs which were just created from a random peer
	secondReceiverWallet := wallet.New()
	secondReceiverAddresses := make([]string, depositCount)
	secondReceiverExpectedBalances := map[string]map[balance.Color]int64{}
	secondReceiverExpectedTransactions := map[string]*tests.ExpectedTransaction{}
	for i := 0; i < depositCount; i++ {
		addr := secondReceiverWallet.Seed().Address(uint64(i))
		tx := transaction.New(
			transaction.NewInputs(transaction.NewOutputID(firstReceiver.Seed().Address(uint64(i)), tx.ID())),
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				addr: {{Value: deposit, Color: balance.ColorIOTA}},
			}),
		)
		secondReceiverAddresses[i] = addr.String()
		tx = tx.Sign(signaturescheme.ED25519(*firstReceiver.Seed().KeyPair(uint64(i))))
		txID, err := n.Peers()[rand.Intn(len(n.Peers()))].SendTransaction(tx.Bytes())
		require.NoError(t, err)

		utilsTx := utils.ParseTransaction(tx)
		secondReceiverExpectedBalances[addr.String()] = map[balance.Color]int64{balance.ColorIOTA: deposit}
		secondReceiverExpectedTransactions[txID] = &tests.ExpectedTransaction{
			Inputs: &utilsTx.Inputs, Outputs: &utilsTx.Outputs, Signature: &utilsTx.Signature,
		}
	}

	// wait again some network delays for the transactions to materialize
	log.Println("waiting 2.5 avg. network delays")
	time.Sleep(valuetransfers.DefaultAverageNetworkDelay*2 + valuetransfers.DefaultAverageNetworkDelay/2)
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
