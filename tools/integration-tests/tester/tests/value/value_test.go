package value

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/createnftoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/delegateoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/destroynftoptions"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
)

// TestValueTransactionPersistence issues transactions on random peers, restarts them and checks for persistence after restart.
func TestValueTransactionPersistence(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotInfo := tests.EqualSnapshotDetails
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Activity:    true, // we need to issue regular activity messages
		Snapshots:   []framework.SnapshotInfo{snapshotInfo},
		PeerMaster:  true,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	for i, p := range n.Peers() {
		resp, _ := p.Info()
		t.Logf("node %d mana: %v acc %v\n", i, resp.Mana.Consensus, resp.Mana.Access)
	}
	// check consensus mana
	// faucet node has zero mana because it pledges its mana to `1111111` node
	require.Eventually(t, func() bool {
		return tests.Mana(t, n.Peers()[0]).Consensus == 0
	}, tests.Timeout, tests.Tick)
	// the rest of the nodes should have mana as in snapshot
	for i, peer := range n.Peers()[1:] {
		if snapshotInfo.PeersAmountsPledged[i] > 0 {
			require.Eventually(t, func() bool {
				return tests.Mana(t, peer).Consensus > 0
			}, tests.Timeout, tests.Tick)
		}
		require.EqualValues(t, snapshotInfo.PeersAmountsPledged[i], tests.Mana(t, peer).Consensus)
	}

	faucet, nonFaucetPeers := n.Peers()[0], n.Peers()[1:]
	tokensPerRequest := uint64(faucet.Config().Faucet.TokensPerRequest)
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	addrBalance := make(map[string]map[ledgerstate.Color]uint64)

	// request funds from faucet
	for _, peer := range nonFaucetPeers {
		addr := peer.Address(0)
		tests.SendFaucetRequest(t, peer, addr)
		addrBalance[addr.Base58()] = map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: tokensPerRequest}
	}

	// wait for messages to be gossiped
	for _, peer := range nonFaucetPeers {
		require.Eventually(t, func() bool {
			return tests.Balance(t, peer, peer.Address(0), ledgerstate.ColorIOTA) == tokensPerRequest
		}, tests.Timeout, tests.Tick)
	}

	// send IOTA tokens from every peer
	expectedStates := make(map[string]tests.ExpectedState)
	for _, peer := range nonFaucetPeers {
		txID, err := tests.SendTransaction(t, peer, peer, ledgerstate.ColorIOTA, 100, tests.TransactionConfig{ToAddressIndex: 1}, addrBalance)
		require.NoError(t, err)
		expectedStates[txID] = tests.ExpectedState{GradeOfFinality: tests.GoFPointer(gof.High)}
	}

	// check ledger state
	tests.RequireGradeOfFinalityEqual(t, n.Peers(), expectedStates, tests.Timeout, tests.Tick)
	tests.RequireBalancesEqual(t, n.Peers(), addrBalance)

	// send colored tokens from every peer
	for _, peer := range nonFaucetPeers {
		txID, err := tests.SendTransaction(t, peer, peer, ledgerstate.ColorMint, 100, tests.TransactionConfig{ToAddressIndex: 2}, addrBalance)
		require.NoError(t, err)
		expectedStates[txID] = tests.ExpectedState{GradeOfFinality: tests.GoFPointer(gof.High)}
	}

	tests.RequireGradeOfFinalityEqual(t, n.Peers(), expectedStates, tests.Timeout, tests.Tick)
	tests.RequireBalancesEqual(t, n.Peers(), addrBalance)

	log.Printf("Restarting %d peers...", len(nonFaucetPeers))
	for _, peer := range nonFaucetPeers {
		require.NoError(t, peer.Restart(ctx))
	}
	log.Println("Restarting peers... done")

	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	tests.RequireGradeOfFinalityEqual(t, n.Peers(), expectedStates, tests.Timeout, tests.Tick)
	tests.RequireBalancesEqual(t, n.Peers(), addrBalance)
}

// TestValueAliasPersistence creates an alias output, restarts all nodes, and checks whether the output is persisted.
func TestValueAliasPersistence(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotInfo := tests.EqualSnapshotDetails
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Activity:    true, // we need to issue regular activity messages
		PeerMaster:  true,
		Snapshots:   []framework.SnapshotInfo{snapshotInfo},
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// check consensus mana
	// faucet node has zero mana because it pledges its mana to `1111111` node
	require.Eventually(t, func() bool {
		return tests.Mana(t, n.Peers()[0]).Consensus == 0
	}, tests.Timeout, tests.Tick)
	// the rest of the nodes should have mana as in snapshot
	for i, peer := range n.Peers()[1:] {
		if snapshotInfo.PeersAmountsPledged[i] > 0 {
			require.Eventually(t, func() bool {
				return tests.Mana(t, peer).Consensus > 0
			}, tests.Timeout, tests.Tick)
		}
		require.EqualValues(t, snapshotInfo.PeersAmountsPledged[i], tests.Mana(t, peer).Consensus)
	}

	faucet, peer := n.Peers()[0], n.Peers()[1]
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	// create a wallet that connects to a random peer
	w := wallet.New(wallet.WebAPI(peer.BaseURL()), wallet.FaucetPowDifficulty(faucet.Config().Faucet.PowDifficulty))

	err = w.RequestFaucetFunds(true)
	require.NoError(t, err)

	tx, aliasID, err := w.CreateNFT(
		createnftoptions.ImmutableData([]byte("can't touch this")),
		createnftoptions.WaitForConfirmation(true),
	)
	require.NoError(t, err)

	expectedState := map[string]tests.ExpectedState{
		tx.ID().Base58(): {
			GradeOfFinality: tests.GoFPointer(gof.High),
		},
	}
	tests.RequireGradeOfFinalityEqual(t, n.Peers(), expectedState, tests.Timeout, tests.Tick)

	aliasOutputID := checkAliasOutputOnAllPeers(t, n.Peers(), aliasID)

	// restart all nodes
	for _, peer := range n.Peers()[1:] {
		require.NoError(t, peer.Restart(ctx))
	}

	// wait for peers to start
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// check if nodes still have the outputs and transaction
	tests.RequireGradeOfFinalityEqual(t, n.Peers(), expectedState, tests.Timeout, tests.Tick)

	checkAliasOutputOnAllPeers(t, n.Peers(), aliasID)

	_, err = w.DestroyNFT(destroynftoptions.Alias(aliasID.Base58()), destroynftoptions.WaitForConfirmation(true))
	require.NoError(t, err)

	// check if all nodes destroyed it
	for _, peer := range n.Peers() {
		outputMetadata, err := peer.GetOutputMetadata(aliasOutputID.Base58())
		require.NoError(t, err)
		// it has been spent
		require.True(t, outputMetadata.ConsumerCount > 0)

		resp, err := peer.GetAddressUnspentOutputs(aliasID.Base58())
		require.NoError(t, err)
		// there should be no outputs
		require.True(t, len(resp.Outputs) == 0)
	}
}

// TestValueAliasDelegation tests if a delegation output can be used to refresh mana.
func TestValueAliasDelegation(t *testing.T) {
	t.Skip("Value Alias Delegation test needs to be fixed.")
	snapshotInfo := tests.EqualSnapshotDetails
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: true,
		Faucet:      true,
		Activity:    true, // we need to issue regular activity messages
		PeerMaster:  true,
		Snapshots:   []framework.SnapshotInfo{snapshotInfo},
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)

	// check consensus mana
	// faucet node has zero mana because it pledges its mana to `1111111` node
	require.Eventually(t, func() bool {
		return tests.Mana(t, n.Peers()[0]).Consensus == 0
	}, tests.Timeout, tests.Tick)
	// the rest of the nodes should have mana as in snapshot
	for i, peer := range n.Peers()[1:] {
		if snapshotInfo.PeersAmountsPledged[i] > 0 {
			require.Eventually(t, func() bool {
				return tests.Mana(t, peer).Consensus > 0
			}, tests.Timeout, tests.Tick)
		}
		require.EqualValues(t, snapshotInfo.PeersAmountsPledged[i], tests.Mana(t, peer).Consensus)
	}

	faucet, peer := n.Peers()[0], n.Peers()[1]
	tests.AwaitInitialFaucetOutputsPrepared(t, faucet, n.Peers())

	// create a wallet that connects to a random peer
	w := wallet.New(wallet.WebAPI(peer.BaseURL()), wallet.FaucetPowDifficulty(faucet.Config().Faucet.PowDifficulty))

	err = w.RequestFaucetFunds(true)
	require.NoError(t, err)

	dumbWallet := createWallets(1)[0]
	delegationAddress := dumbWallet.address
	_, delegationIDs, err := w.DelegateFunds(
		delegateoptions.Destination(address.Address{AddressBytes: delegationAddress.Array()}, map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 1000}),
		delegateoptions.WaitForConfirmation(true),
	)
	require.NoError(t, err)

	delegatedAliasOutputID := ledgerstate.OutputID{}
	delegatedAliasOutput := &ledgerstate.AliasOutput{}
	for i, peer := range n.Peers() {
		resp, err := peer.GetAddressUnspentOutputs(delegationIDs[0].Base58())
		require.NoError(t, err)
		// there should be only this output
		require.True(t, len(resp.Outputs) == 1)
		shouldBeAliasOutput, err := resp.Outputs[0].ToLedgerstateOutput()
		require.NoError(t, err)
		require.Equal(t, ledgerstate.AliasOutputType, shouldBeAliasOutput.Type())
		alias, ok := shouldBeAliasOutput.(*ledgerstate.AliasOutput)
		require.True(t, ok)
		require.Equal(t, delegationIDs[0].Base58(), alias.GetAliasAddress().Base58())
		require.True(t, alias.IsDelegated())
		switch i {
		case 0:
			delegatedAliasOutputID = alias.ID()
			delegatedAliasOutput = alias
		default:
			require.Equal(t, delegatedAliasOutputID.Base58(), alias.ID().Base58())
			require.Equal(t, delegatedAliasOutput.Bytes(), alias.Bytes())
		}
	}

	aManaReceiver, err := identity.RandomID()
	require.NoError(t, err)
	cManaReceiver, err := identity.RandomID()
	require.NoError(t, err)

	// let's try to "refresh mana"
	nextOutput := delegatedAliasOutput.NewAliasOutputNext(false)
	essence := ledgerstate.NewTransactionEssence(0, time.Now(),
		aManaReceiver, cManaReceiver,
		ledgerstate.NewInputs(ledgerstate.NewUTXOInput(delegatedAliasOutputID)),
		ledgerstate.NewOutputs(nextOutput))
	tx := ledgerstate.NewTransaction(essence, dumbWallet.unlockBlocks(essence))
	_, err = peer.PostTransaction(tx.Bytes())
	require.NoError(t, err)

	tests.RequireGradeOfFinalityEqual(t, n.Peers(), map[string]tests.ExpectedState{
		tx.ID().Base58(): {
			GradeOfFinality: tests.GoFPointer(gof.High),
		},
	}, tests.Timeout, tests.Tick)

	aManaReceiverCurrMana, err := peer.GetManaFullNodeID(base58.Encode(aManaReceiver.Bytes()))
	require.NoError(t, err)
	cManaReceiverCurrMana, err := peer.GetManaFullNodeID(base58.Encode(cManaReceiver.Bytes()))
	require.NoError(t, err)

	// check that the pledge actually worked
	require.True(t, aManaReceiverCurrMana.Access > 0)
	require.True(t, cManaReceiverCurrMana.Consensus > 0)
}

func checkAliasOutputOnAllPeers(t *testing.T, peers []*framework.Node, aliasAddr *ledgerstate.AliasAddress) ledgerstate.OutputID {
	aliasOutputID := ledgerstate.OutputID{}

	for i, peer := range peers {
		resp, err := peer.GetAddressUnspentOutputs(aliasAddr.Base58())
		require.NoError(t, err)
		// there should be only this output
		require.True(t, len(resp.Outputs) == 1)
		shouldBeAliasOutput, err := resp.Outputs[0].ToLedgerstateOutput()
		require.NoError(t, err)
		require.Equal(t, ledgerstate.AliasOutputType, shouldBeAliasOutput.Type())
		alias, ok := shouldBeAliasOutput.(*ledgerstate.AliasOutput)
		require.True(t, ok)
		require.Equal(t, aliasAddr.Base58(), alias.GetAliasAddress().Base58())
		switch i {
		case 0:
			aliasOutputID = alias.ID()
		default:
			require.Equal(t, aliasOutputID.Base58(), alias.ID().Base58())
		}
	}
	return aliasOutputID
}

type simpleWallet struct {
	keyPair ed25519.KeyPair
	address *ledgerstate.ED25519Address
}

func (s simpleWallet) privateKey() ed25519.PrivateKey {
	return s.keyPair.PrivateKey
}

func (s simpleWallet) publicKey() ed25519.PublicKey {
	return s.keyPair.PublicKey
}

func createWallets(n int) []simpleWallet {
	wallets := make([]simpleWallet, n)
	for i := 0; i < n; i++ {
		kp := ed25519.GenerateKeyPair()
		wallets[i] = simpleWallet{
			kp,
			ledgerstate.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (s simpleWallet) sign(txEssence *ledgerstate.TransactionEssence) *ledgerstate.ED25519Signature {
	return ledgerstate.NewED25519Signature(s.publicKey(), s.privateKey().Sign(txEssence.Bytes()))
}

func (s simpleWallet) unlockBlocks(txEssence *ledgerstate.TransactionEssence) []ledgerstate.UnlockBlock {
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(s.sign(txEssence))
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

func createGenesisWallet(node *framework.Node) *wallet.Wallet {
	webConn := wallet.GenericConnector(wallet.NewWebConnector(node.BaseURL()))
	return wallet.New(wallet.Import(walletseed.NewSeed(framework.GenesisSeedBytes), 0, []bitmask.BitMask{}, nil), webConn)
}
