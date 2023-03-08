package value

import (
	"context"
	"log"
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/createnftoptions"
	"github.com/iotaledger/goshimmer/client/wallet/packages/destroynftoptions"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/tests"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/ds/bitmask"
	"github.com/iotaledger/hive.go/lo"
)

// TestValueTransactionPersistence issues transactions on random peers, restarts them and checks for persistence after restart.
func TestValueTransactionPersistence(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotOptions := tests.EqualSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)
	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: false,
		Faucet:      true,
		Activity:    true, // we need to issue regular activity blocks
		Snapshot:    snapshotOptions,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)
	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	for i, p := range n.Peers() {
		resp, _ := p.Info()
		t.Logf("node %d mana: %v acc %v\n", i, resp.Mana.Consensus, resp.Mana.Access)
	}

	faucet, nonFaucetPeers := n.Peers()[0], n.Peers()[1:]

	// check consensus mana: all nodes should have equal mana
	require.Eventually(t, func() bool {
		return tests.Mana(t, faucet).Consensus > 0
	}, tests.Timeout, tests.Tick)
	require.EqualValues(t, snapshotInfo.GenesisTokenAmount, tests.Mana(t, faucet).Consensus)

	for i, peer := range nonFaucetPeers {
		if snapshotInfo.PeersAmountsPledged[i] > 0 {
			require.Eventually(t, func() bool {
				return tests.Mana(t, peer).Consensus > 0
			}, tests.Timeout, tests.Tick)
		}
		require.EqualValues(t, snapshotInfo.PeersAmountsPledged[i], tests.Mana(t, peer).Consensus)
	}

	tokensPerRequest := uint64(faucet.Config().Faucet.TokensPerRequest)
	addrBalance := make(map[string]map[devnetvm.Color]uint64)

	// request funds from faucet
	for _, peer := range nonFaucetPeers {
		addr := peer.Address(0)
		tests.SendFaucetRequest(t, peer, addr)
		addrBalance[addr.Base58()] = map[devnetvm.Color]uint64{devnetvm.ColorIOTA: tokensPerRequest}
	}

	// wait for blocks to be gossiped
	for _, peer := range nonFaucetPeers {
		require.Eventually(t, func() bool {
			return tests.Balance(t, peer, peer.Address(0), devnetvm.ColorIOTA) == tokensPerRequest
		}, tests.Timeout, tests.Tick)
	}

	// send IOTA tokens from every peer
	expectedStates := make(map[string]tests.ExpectedState)
	for _, peer := range nonFaucetPeers {
		txID, err := tests.SendTransaction(t, peer, peer, devnetvm.ColorIOTA, 100, tests.TransactionConfig{ToAddressIndex: 1}, addrBalance)
		require.NoError(t, err)
		expectedStates[txID] = tests.ExpectedState{ConfirmationState: confirmation.Accepted}
	}

	// check ledger state
	tests.RequireConfirmationStateEqual(t, n.Peers(), expectedStates, tests.Timeout, tests.Tick)
	tests.RequireBalancesEqual(t, n.Peers(), addrBalance)

	// send colored tokens from every peer
	for _, peer := range nonFaucetPeers {
		txID, err := tests.SendTransaction(t, peer, peer, devnetvm.ColorMint, 100, tests.TransactionConfig{ToAddressIndex: 2}, addrBalance)
		require.NoError(t, err)
		expectedStates[txID] = tests.ExpectedState{ConfirmationState: confirmation.Accepted}
	}

	tests.RequireConfirmationStateEqual(t, n.Peers(), expectedStates, tests.Timeout, tests.Tick)
	tests.RequireBalancesEqual(t, n.Peers(), addrBalance)

	// TODO: restarting a node not supported yet
	return
	log.Printf("Restarting %d peers...", len(nonFaucetPeers))
	for _, peer := range nonFaucetPeers {
		require.NoError(t, peer.Restart(ctx))
	}
	log.Println("Restarting peers... done")

	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	tests.RequireConfirmationStateEqual(t, n.Peers(), expectedStates, tests.Timeout, tests.Tick)
	tests.RequireBalancesEqual(t, n.Peers(), addrBalance)
}

// TestValueAliasPersistence creates an alias output, restarts all nodes, and checks whether the output is persisted.
func TestValueAliasPersistence(t *testing.T) {
	ctx, cancel := tests.Context(context.Background(), t)
	defer cancel()
	snapshotOptions := tests.EqualSnapshotOptions
	snapshotInfo := snapshotcreator.NewOptions(snapshotOptions...)

	n, err := f.CreateNetwork(ctx, t.Name(), 4, framework.CreateNetworkConfig{
		StartSynced: false,
		Faucet:      true,
		Activity:    true, // we need to issue regular activity blocks
		Snapshot:    snapshotOptions,
	}, tests.CommonSnapshotConfigFunc(t, snapshotInfo))
	require.NoError(t, err)
	defer tests.ShutdownNetwork(ctx, t, n)
	log.Println("Bootstrapping network...")
	tests.BootstrapNetwork(t, n)
	log.Println("Bootstrapping network... done")

	faucet, nonFaucetPeers := n.Peers()[0], n.Peers()[1:]

	// check consensus mana: all nodes should have equal mana
	require.Eventually(t, func() bool {
		return tests.Mana(t, faucet).Consensus > 0
	}, tests.Timeout, tests.Tick)
	require.EqualValues(t, snapshotInfo.GenesisTokenAmount, tests.Mana(t, faucet).Consensus)

	for i, peer := range nonFaucetPeers {
		if snapshotInfo.PeersAmountsPledged[i] > 0 {
			require.Eventually(t, func() bool {
				return tests.Mana(t, peer).Consensus > 0
			}, tests.Timeout, tests.Tick)
		}
		require.EqualValues(t, snapshotInfo.PeersAmountsPledged[i], tests.Mana(t, peer).Consensus)
	}

	// create a wallet that connects to a random peer
	w := wallet.New(wallet.WebAPI(nonFaucetPeers[0].BaseURL()), wallet.FaucetPowDifficulty(faucet.Config().Faucet.PowDifficulty))

	err = w.RequestFaucetFunds(true)
	require.NoError(t, err)

	tx, aliasID, err := w.CreateNFT(
		createnftoptions.ImmutableData([]byte("can't touch this")),
		createnftoptions.WaitForConfirmation(true),
	)
	require.NoError(t, err)

	expectedState := map[string]tests.ExpectedState{
		tx.ID().Base58(): {
			ConfirmationState: confirmation.Accepted,
		},
	}
	tests.RequireConfirmationStateEqual(t, n.Peers(), expectedState, tests.Timeout, tests.Tick)

	aliasOutputID := checkAliasOutputOnAllPeers(t, n.Peers(), aliasID)
	// TODO: restarting a node not supported yet
	return
	// restart all nodes
	for _, peer := range n.Peers()[1:] {
		require.NoError(t, peer.Restart(ctx))
	}

	// wait for peers to start
	err = n.DoManualPeering(ctx)
	require.NoError(t, err)

	// check if nodes still have the outputs and transaction
	tests.RequireConfirmationStateEqual(t, n.Peers(), expectedState, tests.Timeout, tests.Tick)

	checkAliasOutputOnAllPeers(t, n.Peers(), aliasID)

	_, err = w.DestroyNFT(destroynftoptions.Alias(aliasID.Base58()), destroynftoptions.WaitForConfirmation(true))
	require.NoError(t, err)

	// check if all nodes destroyed it
	for _, peer := range n.Peers() {
		outputMetadata, err := peer.GetOutputMetadata(aliasOutputID.Base58())
		require.NoError(t, err)
		// it has been spent
		require.NotEmpty(t, outputMetadata.FirstConsumer)

		resp, err := peer.GetAddressOutputs(aliasID.Base58())
		require.NoError(t, err)
		// there should be no outputs
		require.True(t, len(resp.UnspentOutputs) == 0)
	}
}

func checkAliasOutputOnAllPeers(t *testing.T, peers []*framework.Node, aliasAddr *devnetvm.AliasAddress) utxo.OutputID {
	aliasOutputID := utxo.OutputID{}

	for i, peer := range peers {
		resp, err := peer.GetAddressOutputs(aliasAddr.Base58())
		require.NoError(t, err)
		// there should be only this output
		require.True(t, len(resp.UnspentOutputs) == 1)
		shouldBeAliasOutput, err := resp.UnspentOutputs[0].ToLedgerstateOutput()
		require.NoError(t, err)
		require.Equal(t, devnetvm.AliasOutputType, shouldBeAliasOutput.Type())
		alias, ok := shouldBeAliasOutput.(*devnetvm.AliasOutput)
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
	address *devnetvm.ED25519Address
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
			devnetvm.NewED25519Address(kp.PublicKey),
		}
	}
	return wallets
}

func (s simpleWallet) sign(txEssence *devnetvm.TransactionEssence) *devnetvm.ED25519Signature {
	return devnetvm.NewED25519Signature(s.publicKey(), s.privateKey().Sign(lo.PanicOnErr(txEssence.Bytes())))
}

func (s simpleWallet) unlockBlocks(txEssence *devnetvm.TransactionEssence) []devnetvm.UnlockBlock {
	unlockBlock := devnetvm.NewSignatureUnlockBlock(s.sign(txEssence))
	unlockBlocks := make([]devnetvm.UnlockBlock, len(txEssence.Inputs()))
	for i := range txEssence.Inputs() {
		unlockBlocks[i] = unlockBlock
	}
	return unlockBlocks
}

func createGenesisWallet(node *framework.Node) *wallet.Wallet {
	webConn := wallet.GenericConnector(wallet.NewWebConnector(node.BaseURL()))
	return wallet.New(wallet.Import(walletseed.NewSeed(framework.GenesisSeedBytes), 0, []bitmask.BitMask{}, nil), webConn)
}
