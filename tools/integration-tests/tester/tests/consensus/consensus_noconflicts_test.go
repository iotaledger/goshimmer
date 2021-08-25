package consensus

import (
	"context"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestConsensusNoConflicts issues valid non-conflicting value objects and then checks
// whether the ledger of every peer reflects the same correct state.
func TestConsensusNoConflicts(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Autopeering: true,
		Activity:    true,
	})
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	faucet := n.Peers()[0]
	// this is necessary as otherwise we are creating a tx spending from genesis concurrently
	// to the faucet creating a tx for the prepared outputs it does per default
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	genesisSeed := seed.NewSeed(framework.GenesisSeed)
	genesisAddr := genesisSeed.Address(0).Address()
	unspentOutputs, err := faucet.PostAddressUnspentOutputs([]string{genesisAddr.Base58()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", n.Peers()[0].String())
	genesisOutput, err := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	genesisBalance, exist := genesisOutput.Balances().Get(ledgerstate.ColorIOTA)
	assert.True(t, exist)
	input := ledgerstate.NewUTXOInput(genesisOutput.ID())

	firstReceiver := seed.NewSeed()
	const depositCount = 10
	deposit := genesisBalance / depositCount
	firstReceiverDepositAddrs := make([]ledgerstate.Address, depositCount)
	firstReceiverDepositOutputs := make(map[ledgerstate.Address]*ledgerstate.ColoredBalances)
	firstReceiverExpectedBalances := make(map[string]map[ledgerstate.Color]uint64)
	for i := 0; i < depositCount; i++ {
		addr := firstReceiver.Address(uint64(i)).Address()
		firstReceiverDepositAddrs[i] = addr
		firstReceiverDepositOutputs[addr] = ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: deposit})
		firstReceiverExpectedBalances[addr.Base58()] = map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: deposit}
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

	// Wait for the approval weight to build up via the sync beacon.
	log.Println("check that the transaction is finalized/confirmed by all peers")
	tests.RequireGradeOfFinalityEqual(t, n.Peers(), map[string]tests.ExpectedState{
		txID: {
			GradeOfFinality: tests.GoFPointer(gof.High),
		},
	}, tests.Timeout, tests.Tick)
	tests.RequireTransactionsEqual(t, n.Peers(), map[string]*tests.ExpectedTransaction{
		txID: {Inputs: utilsTx.Inputs, Outputs: utilsTx.Outputs, UnlockBlocks: utilsTx.UnlockBlocks},
	})

	// since we just issued a transaction spending the genesis output, there
	// shouldn't be any UTXOs on the genesis address anymore
	log.Println("checking that genesis has no UTXOs")
	tests.RequireNoUnspentOutputs(t, n.Peers(), genesisAddr)

	// check balances on peers
	log.Println("ensure that all the peers have the same ledger state")
	tests.RequireBalancesEqual(t, n.Peers(), firstReceiverExpectedBalances)

	// issue transactions spending all the outputs which were just created from a random peer
	secondReceiverSeed := seed.NewSeed()
	secondReceiverAddresses := make([]string, depositCount)
	secondReceiverExpectedBalances := map[string]map[ledgerstate.Color]uint64{}
	secondReceiverExpectedStates := map[string]tests.ExpectedState{}
	secondReceiverExpectedTransactions := map[string]*tests.ExpectedTransaction{}

	for i := 0; i < depositCount; i++ {
		addr := secondReceiverSeed.Address(uint64(i)).Address()
		input := ledgerstate.NewUTXOInput(tx1.Essence().Outputs()[tests.OutputIndex(tx1, firstReceiver.Address(uint64(i)).Address())].ID())
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

		secondReceiverExpectedBalances[addr.Base58()] = map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: deposit}
		secondReceiverExpectedStates[txID] = tests.ExpectedState{
			GradeOfFinality: tests.GoFPointer(gof.High),
		}
		secondReceiverExpectedTransactions[txID] = &tests.ExpectedTransaction{
			Inputs: utilsTx.Inputs, Outputs: utilsTx.Outputs, UnlockBlocks: utilsTx.UnlockBlocks,
		}
	}

	log.Println("checking that the 2nd batch transactions are finalized/confirmed")
	tests.RequireGradeOfFinalityEqual(t, n.Peers(), secondReceiverExpectedStates, tests.Timeout, tests.Tick)
	tests.RequireTransactionsEqual(t, n.Peers(), secondReceiverExpectedTransactions)
	log.Println("checking that first set of addresses contain no UTXOs")
	tests.RequireNoUnspentOutputs(t, n.Peers(), firstReceiverDepositAddrs...)

	log.Println("check that the 2nd batch of receive addresses is the same on all peers")
	tests.RequireBalancesEqual(t, n.Peers(), secondReceiverExpectedBalances)
}
