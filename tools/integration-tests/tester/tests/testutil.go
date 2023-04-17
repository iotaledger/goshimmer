package tests

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/hive.go/runtime/options"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
)

var faucetPoWDifficulty = framework.PeerConfig().Faucet.PowDifficulty

const (
	// Timeout denotes the default condition polling timout duration.
	Timeout = 1 * time.Minute
	// Tick denotes the default condition polling tick time.
	Tick = 500 * time.Millisecond

	shutdownGraceTime = time.Minute
)

// OrphanageSnapshotOptions defines snapshot options for orphanage test scenario.
var OrphanageSnapshotOptions = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithFilePath("/assets/dynamic_snapshots/orphanage_snapshot.bin"),
	snapshotcreator.WithGenesisTokenAmount(0),
	snapshotcreator.WithPeersSeedBase58([]string{
		"3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E", // FZ6xmPZX
		"GtKSdqanb4mokUBjAf9JZmsSqWzWjzzw57mRR56LjfBL", // H6jzPnLbjsh
		"CmFVE14Yh9rqn2FrXD8s7ybRoRN5mUnqQxLAuD5HF2em", // JHxvcap7xhv
		"DuJuWE3hisFrFK1HmrXkd9FSsNNWbw58JcQnKdBn6TdN", // 7rRpyEGU7Sf
	}),
	snapshotcreator.WithPeersAmountsPledged([]uint64{2_500_000_000_000_000, 2_500_000_000_000_000, 2_500_000_000_000_000, 10}),
}

// EqualSnapshotOptions defines snapshot options for equal test scenario.
var EqualSnapshotOptions = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithFilePath("/assets/dynamic_snapshots/equal_snapshot.bin"),
	snapshotcreator.WithGenesisTokenAmount(2_500_000_000_000_000),
	snapshotcreator.WithPeersSeedBase58([]string{
		"GtKSdqanb4mokUBjAf9JZmsSqWzWjzzw57mRR56LjfBL", // H6jzPnLbjsh
		"CmFVE14Yh9rqn2FrXD8s7ybRoRN5mUnqQxLAuD5HF2em", // JHxvcap7xhv
		"DuJuWE3hisFrFK1HmrXkd9FSsNNWbw58JcQnKdBn6TdN", // 7rRpyEGU7Sf
		"3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E", // FZ6xmPZX
	}),
	snapshotcreator.WithPeersAmountsPledged([]uint64{2_500_000_000_000_000, 2_500_000_000_000_000, 2_500_000_000_000_000, 2_500_000_000_000_000}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"B45CgJeL9rfigCNXdkReZoVmK4RJqw4E81zYuETA4zJC"}), // pk for seed GtKSdqanb4mokUBjAf9JZmsSqWzWjzzw57mRR56LjfBL
}

var ConsensusSnapshotOptions = []options.Option[snapshotcreator.Options]{
	snapshotcreator.WithFilePath("/assets/dynamic_snapshots/consensus_snapshot.bin"),
	snapshotcreator.WithGenesisTokenAmount(800_000),
	snapshotcreator.WithPeersSeedBase58([]string{
		"Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8", // jnaC6ZyWuw
		"HUH4rmxUxMZBBtHJ4QM5Ts6s8DP3HnFpChejntnCxto2", // iNvPFvkfSDp
		"CmFVE14Yh9rqn2FrXD8s7ybRoRN5mUnqQxLAuD5HF2em", // JHxvcap7xhv
		"DuJuWE3hisFrFK1HmrXkd9FSsNNWbw58JcQnKdBn6TdN", // 7rRpyEGU7Sf
	}),
	snapshotcreator.WithPeersAmountsPledged([]uint64{1_600_000, 800_000, 800_000, 800_000, 800_000}),
	snapshotcreator.WithInitialAttestationsBase58([]string{"3kwsHfLDb7ifuxLbyMZneXq3s5heRWnXKKGPAARJDaUE"}), // pk for seed Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8
}

// GetIdentSeed returns decoded seed bytes for the supplied SnapshotInfo and peer index
func GetIdentSeed(t *testing.T, snapshotOptions *snapshotcreator.Options, peerIndex int) []byte {
	seedBytes, err := base58.Decode(snapshotOptions.PeersSeedBase58[peerIndex])
	require.NoError(t, err)
	return seedBytes
}

// CommonSnapshotConfigFunc returns a peer configuration altering function that uses the specified Snapshot information for all peers.
// If a cfgFunc is provided, further manipulation of the base config for every peer is possible.
func CommonSnapshotConfigFunc(t *testing.T, snapshotOptions *snapshotcreator.Options, cfgFunc ...framework.CfgAlterFunc) framework.CfgAlterFunc {
	return func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
		conf.Protocol.Snapshot.Path = snapshotOptions.FilePath

		require.Lessf(t, peerIndex, len(snapshotOptions.PeersSeedBase58), "index=%d out of range for peerSeeds=%d", peerIndex, len(snapshotOptions.PeersSeedBase58))
		conf.Seed = GetIdentSeed(t, snapshotOptions, peerIndex)

		if len(cfgFunc) > 0 {
			conf = cfgFunc[0](peerIndex, isPeerMaster && peerIndex == 0, conf)
		}

		return conf
	}
}

// DataBlockSent defines a struct to identify from which issuer a data block was sent.
type DataBlockSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

// TransactionConfig defines the configuration for a transaction.
type TransactionConfig struct {
	FromAddressIndex      uint64
	ToAddressIndex        uint64
	AccessManaPledgeID    identity.ID
	ConsensusManaPledgeID identity.ID
}

// Context creates a new context that matches the test deadline.
func Context(ctx context.Context, t *testing.T) (context.Context, context.CancelFunc) {
	if d, ok := t.Deadline(); ok {
		return context.WithDeadline(ctx, d.Add(-shutdownGraceTime))
	}
	return context.WithCancel(ctx)
}

// Synced returns whether node is synchronized.
func Synced(t *testing.T, node *framework.Node) bool {
	info, err := node.Info()
	require.NoError(t, err)
	return info.TangleTime.Synced
}

// Bootstrapped returns whether node is bootstrapped.
func Bootstrapped(t *testing.T, node *framework.Node) bool {
	info, err := node.Info()
	require.NoError(t, err)
	return info.TangleTime.Bootstrapped
}

func BootstrapNetwork(t *testing.T, n *framework.Network) {
	require.Eventually(t, func() bool {
		bootstrappedPeers := lo.Filter(n.Peers(), func(p *framework.Node) bool {
			return p.Config().IgnoreBootstrappedFlag || Bootstrapped(t, p)
		})

		SendDataBlocks(t, bootstrappedPeers, len(bootstrappedPeers))
		for _, p := range n.Peers() {
			if !Bootstrapped(t, p) {
				return false
			}
		}
		return true
	}, Timeout*2, Tick)
}

// Mana returns the mana reported by node.
func Mana(t *testing.T, node *framework.Node) jsonmodels.Mana {
	info, err := node.Info()
	require.NoError(t, err)
	return info.Mana
}

// AddressUnspentOutputs returns the unspent outputs on address.
func AddressUnspentOutputs(t *testing.T, node *framework.Node, address devnetvm.Address, numOfExpectedOuts int) []jsonmodels.WalletOutput {
	resp, err := node.PostAddressUnspentOutputs([]string{address.Base58()})
	require.NoErrorf(t, err, "node=%s, address=%s, PostAddressUnspentOutputs failed", node, address.Base58())
	require.Lenf(t, resp.UnspentOutputs, numOfExpectedOuts, "invalid response")
	require.Equalf(t, address.Base58(), resp.UnspentOutputs[0].Address.Base58, "invalid response")

	return resp.UnspentOutputs[0].Outputs
}

// Balance returns the total balance of color at address.
func Balance(t *testing.T, node *framework.Node, address devnetvm.Address, color devnetvm.Color) uint64 {
	unspentOutputs := AddressUnspentOutputs(t, node, address, 1)
	fmt.Println("Balance: ", node.ID(), node.Name())

	var sum uint64
	for _, output := range unspentOutputs {
		out, err := output.Output.ToLedgerstateOutput()
		fmt.Println("output: ", out.Address(), out.ID(), out.Balances())
		require.NoError(t, err)
		balance, _ := out.Balances().Get(color)
		fmt.Println("balance: ", balance, color)
		sum += balance
	}
	return sum
}

// SendFaucetRequest sends a data block on a given peer and returns the id and a DataBlockSent struct. By default,
// it pledges mana to the peer making the request.
func SendFaucetRequest(t *testing.T, node *framework.Node, addr devnetvm.Address, manaPledgeIDs ...string) (string, DataBlockSent) {
	nodeID := node.ID().EncodeBase58()
	aManaPledgeID, cManaPledgeID := nodeID, nodeID
	if len(manaPledgeIDs) > 1 {
		aManaPledgeID, cManaPledgeID = manaPledgeIDs[0], manaPledgeIDs[1]
	}

	resp, err := node.BroadcastFaucetRequest(addr.Base58(), faucetPoWDifficulty, aManaPledgeID, cManaPledgeID)
	require.NoErrorf(t, err, "node=%s, address=%s, BroadcastFaucetRequest failed", node, addr.Base58())
	fmt.Println("faucet resp ", resp.ID)
	sent := DataBlockSent{
		id:              resp.ID,
		data:            nil,
		issuerPublicKey: node.Identity.PublicKey().String(),
	}

	// Make sure the block is available on the peer itself and has confirmation.State Pending.
	RequireBlocksAvailable(t, []*framework.Node{node}, map[string]DataBlockSent{sent.id: sent}, Timeout, Tick)

	return resp.ID, sent
}

// region CreateTransaction from outputs //////////////////////////////

// CreateTransactionFromOutputs takes the given utxos inputs and create a transaction that spreads the total input balance
// across the targetAddresses. In order to correctly sign we have a keyPair map that maps a given address to its public key.
// Access and Consensus Mana is pledged to the node we specify.
func CreateTransactionFromOutputs(t *testing.T, manaPledgeID identity.ID, targetAddresses []devnetvm.Address, keyPairs map[string]*ed25519.KeyPair, utxos ...devnetvm.Output) *devnetvm.Transaction {
	// Create Inputs from utxos
	inputs := devnetvm.Inputs{}
	balances := map[devnetvm.Color]uint64{}
	for _, output := range utxos {
		output.Balances().ForEach(func(color devnetvm.Color, balance uint64) bool {
			balances[color] += balance
			return true
		})
		inputs = append(inputs, output.Input())
	}

	// create outputs for each target address
	numberOfOutputs := len(targetAddresses)
	outputs := make(devnetvm.Outputs, numberOfOutputs)
	for i := 0; i < numberOfOutputs; i++ {
		outBalances := map[devnetvm.Color]uint64{}
		for color, balance := range balances {
			// divide by number of outputs to spread funds evenly
			outBalances[color] = balance / uint64(numberOfOutputs)
			if i == numberOfOutputs-1 {
				// on the last iteration add the remainder so all funds are consumed
				outBalances[color] += balance % uint64(numberOfOutputs)
			}
		}
		outputs[i] = devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(outBalances), targetAddresses[i])
	}

	// create tx essence
	txEssence := devnetvm.NewTransactionEssence(0, time.Now(), manaPledgeID,
		manaPledgeID, inputs, devnetvm.NewOutputs(outputs...))

	// create signatures
	unlockBlocks := make([]devnetvm.UnlockBlock, len(inputs))

	for i := 0; i < len(inputs); i++ {
		addressKey := utxos[i].Address().String()
		keyPair := keyPairs[addressKey]
		require.NotNilf(t, keyPair, "missing key pair for address %s", addressKey)
		sig := devnetvm.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(lo.PanicOnErr(txEssence.Bytes())))
		unlockBlocks[i] = devnetvm.NewSignatureUnlockBlock(sig)
	}

	return devnetvm.NewTransaction(txEssence, unlockBlocks)
}

// endregion

// SendDataBlock sends a data block on a given peer and returns the id and a DataBlockSent struct.
func SendDataBlock(t *testing.T, node *framework.Node, data []byte, number int) (string, DataBlockSent) {
	id, err := node.Data(data, 30*time.Second)
	require.NoErrorf(t, err, "node=%s, 'Data' failed with error %s", node, err)

	sent := DataBlockSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            lo.PanicOnErr(payload.NewGenericDataPayload(data).Bytes()),
		issuerPublicKey: node.Identity.PublicKey().String(),
	}
	return id, sent
}

// SendDataBlocks sends a total of numBlocks data blocks and saves the sent block to a map.
// It chooses the peers to send the blocks from in a round-robin fashion.
func SendDataBlocks(t *testing.T, peers []*framework.Node, numBlocks int, idsMap ...map[string]DataBlockSent) map[string]DataBlockSent {
	var result map[string]DataBlockSent
	if len(idsMap) > 0 {
		result = idsMap[0]
	} else {
		result = make(map[string]DataBlockSent, numBlocks)
	}

	for i := 0; i < numBlocks; i++ {
		data := []byte(fmt.Sprintf("Test: %d", i))

		id, sent := SendDataBlock(t, peers[i%len(peers)], data, i)
		result[id] = sent
	}
	return result
}

// SendDataBlocksWithDelay sends a total of numBlocks data blocks, each after a delay interval, and saves the sent block to a map.
// It chooses the peers to send the blocks from in a round-robin fashion.
func SendDataBlocksWithDelay(t *testing.T, peers []*framework.Node, numBlocks int, delay time.Duration, idsMap ...map[string]DataBlockSent) (result map[string]DataBlockSent) {
	if len(idsMap) > 0 {
		result = idsMap[0]
	} else {
		result = make(map[string]DataBlockSent, numBlocks)
	}

	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for i := 0; i < numBlocks; i++ {
		data := []byte(fmt.Sprintf("Test: %d", i))

		id, sent := SendDataBlock(t, peers[i%len(peers)], data, i)
		result[id] = sent
		<-ticker.C
	}

	return
}

// SendTransaction sends a transaction of value and color. It returns the transactionID and the error return by PostTransaction.
// If addrBalance is given the balance mutation are added to that map.
func SendTransaction(t *testing.T, from *framework.Node, to *framework.Node, color devnetvm.Color, value uint64, txConfig TransactionConfig, addrBalance ...map[string]map[devnetvm.Color]uint64) (string, error) {
	inputAddr := from.Seed.Address(txConfig.FromAddressIndex).Address()
	outputAddr := to.Seed.Address(txConfig.ToAddressIndex).Address()

	unspentOutputs := AddressUnspentOutputs(t, from, inputAddr, 1)
	require.NotEmptyf(t, unspentOutputs, "address=%s, no unspent outputs", inputAddr.Base58())

	inputColor := color
	if color == devnetvm.ColorMint {
		inputColor = devnetvm.ColorIOTA
	}
	balance := Balance(t, from, inputAddr, inputColor)
	require.GreaterOrEqualf(t, balance, value, "address=%s, insufficient balance", inputAddr.Base58())

	out, err := unspentOutputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)

	input := devnetvm.NewUTXOInput(out.ID())

	var outputs devnetvm.Outputs
	output := devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
		color: value,
	}), outputAddr)
	outputs = append(outputs, output)

	// handle remainder address
	if balance > value {
		output := devnetvm.NewSigLockedColoredOutput(devnetvm.NewColoredBalances(map[devnetvm.Color]uint64{
			inputColor: balance - value,
		}), inputAddr)
		outputs = append(outputs, output)
	}

	txEssence := devnetvm.NewTransactionEssence(0, time.Now(), txConfig.AccessManaPledgeID, txConfig.ConsensusManaPledgeID, devnetvm.NewInputs(input), devnetvm.NewOutputs(outputs...))
	sig := devnetvm.NewED25519Signature(from.KeyPair(txConfig.FromAddressIndex).PublicKey, from.KeyPair(txConfig.FromAddressIndex).PrivateKey.Sign(lo.PanicOnErr(txEssence.Bytes())))
	unlockBlock := devnetvm.NewSignatureUnlockBlock(sig)
	txn := devnetvm.NewTransaction(txEssence, devnetvm.UnlockBlocks{unlockBlock})

	outputColor := color
	if color == devnetvm.ColorMint {
		mintOutput := txn.Essence().Outputs()[OutputIndex(txn, outputAddr)]
		outputColor = blake2b.Sum256(lo.PanicOnErr(mintOutput.ID().Bytes()))
	}

	// send transaction
	resp, err := from.PostTransaction(lo.PanicOnErr(txn.Bytes()))
	if err != nil {
		return "", err
	}

	if len(addrBalance) > 0 {
		if addrBalance[0][inputAddr.Base58()] == nil {
			addrBalance[0][inputAddr.Base58()] = make(map[devnetvm.Color]uint64)
		}
		addrBalance[0][inputAddr.Base58()][inputColor] -= value
		if addrBalance[0][outputAddr.Base58()] == nil {
			addrBalance[0][outputAddr.Base58()] = make(map[devnetvm.Color]uint64)
		}
		addrBalance[0][outputAddr.Base58()][outputColor] += value
	}
	return resp.TransactionID, nil
}

// RequireBlocksAvailable asserts that all nodes have received BlockIDs in waitFor time, periodically checking each tick.
// Optionally, a ConfirmationState can be specified, which then requires the blocks to reach this ConfirmationState.
func RequireBlocksAvailable(t *testing.T, nodes []*framework.Node, blockIDs map[string]DataBlockSent, waitFor time.Duration, tick time.Duration, accepted ...bool) {
	missing := make(map[identity.ID]*advancedset.AdvancedSet[string], len(nodes))
	for _, node := range nodes {
		missing[node.ID()] = advancedset.New[string]()
		for blockID := range blockIDs {
			missing[node.ID()].Add(blockID)
		}
	}

	condition := func() bool {
		for _, node := range nodes {
			nodeMissing, exists := missing[node.ID()]
			if !exists {
				continue
			}
			for it := nodeMissing.Iterator(); it.HasNext(); {
				blockID := it.Next()
				blk, err := node.GetBlockMetadata(blockID)
				// retry, when the block could not be found
				if errors.Is(err, client.ErrNotFound) {
					log.Printf("node=%s, blockID=%s; block not found", node, blockID)
					continue
				}
				require.NoErrorf(t, err, "node=%s, blockID=%s, 'GetBlockMetadata' failed", node, blockID)

				// retry, if the block has not yet reached the specified ConfirmationState
				if len(accepted) > 0 && accepted[0] {
					if !blk.M.Accepted {
						log.Printf("node=%s, blockID=%s, expected Accepted=true, actual Accepted=%v; ConfirmationState not reached", node, blockID, blk.M.Accepted)
						continue
					}
				}

				require.Equal(t, blockID, blk.ID().Base58())
				nodeMissing.Delete(blockID)
				if nodeMissing.IsEmpty() {
					delete(missing, node.ID())
				}
			}
		}
		return len(missing) == 0
	}

	log.Printf("Waiting for %d blocks to become available...", len(blockIDs))
	require.Eventuallyf(t, condition, waitFor, tick,
		"%d out of %d nodes did not receive all blocks", len(missing), len(nodes))
	log.Println("Waiting for blocks... done")
}

// RequireBlocksOrphaned asserts that all nodes have received BlockIDs and marked them as orphaned in waitFor time, periodically checking each tick.
func RequireBlocksOrphaned(t *testing.T, nodes []*framework.Node, blockIDs map[string]DataBlockSent, waitFor time.Duration, tick time.Duration) {
	missing := make(map[identity.ID]*advancedset.AdvancedSet[string], len(nodes))
	for _, node := range nodes {
		missing[node.ID()] = advancedset.New[string]()
		for blockID := range blockIDs {
			missing[node.ID()].Add(blockID)
		}
	}

	condition := func() bool {
		for _, node := range nodes {
			nodeMissing := missing[node.ID()]
			for it := nodeMissing.Iterator(); it.HasNext(); {
				blockID := it.Next()
				block, err := node.GetBlockMetadata(blockID)
				// retry, when the block could not be found
				if errors.Is(err, client.ErrNotFound) {
					log.Printf("node=%s, blockID=%s; block not found", node, blockID)
					continue
				}

				require.NoErrorf(t, err, "node=%s, blockID=%s, 'GetBlockMetadata' failed", node, blockID)
				require.Equal(t, blockID, block.ID().Base58())
				require.True(t, block.M.Orphaned, "node=%s, blockID=%s, not marked as orphaned", node, blockID)
				nodeMissing.Delete(blockID)
				if nodeMissing.IsEmpty() {
					delete(missing, node.ID())
				}
			}
		}
		return len(missing) == 0
	}

	log.Printf("Waiting for %d blocks to become orphaned...", len(blockIDs))
	require.Eventuallyf(t, condition, waitFor, tick,
		"%d out of %d nodes did not orphan all blocks", len(missing), len(nodes))
	log.Println("Waiting for blocks... done")
}

// RequireBlocksEqual asserts that all nodes return the correct data blocks as specified in blocksByID.
func RequireBlocksEqual(t *testing.T, nodes []*framework.Node, blocksByID map[string]DataBlockSent, waitFor time.Duration, tick time.Duration) {
	condition := func() bool {
		for _, node := range nodes {
			for blockID := range blocksByID {
				resp, err := node.GetBlock(blockID)
				require.NoErrorf(t, err, "node=%s, blockID=%s, 'GetBlock' failed", node, blockID)
				require.Equal(t, blockID, resp.ID)

				respMetadata, err := node.GetBlockMetadata(blockID)
				require.NoErrorf(t, err, "node=%s, blockID=%s, 'BlockMetadata' failed", node, blockID)
				require.Equal(t, blockID, respMetadata.ID().Base58())

				// check for general information
				blkSent := blocksByID[blockID]

				require.Equalf(t, blkSent.issuerPublicKey, resp.IssuerPublicKey, "blockID=%s, issuer=%s not correct issuer in %s.", blkSent.id, blkSent.issuerPublicKey, node)
				if blkSent.data != nil {
					require.Equalf(t, blkSent.data, resp.Payload, "blockID=%s, issuer=%s data not equal in %s.", blkSent.id, blkSent.issuerPublicKey, node)
				}

				if !respMetadata.M.Solid {
					log.Printf("blockID=%s, issuer=%s not solid yet on %s", blkSent.id, blkSent.issuerPublicKey, node)
					return false
				}
			}
		}
		return true
	}

	log.Printf("Waiting for %d blocks to become consistent across peers...", len(blocksByID))
	require.Eventuallyf(t, condition, waitFor, tick, "Nodes are not consistent")
	log.Println("Waiting for blocks... done")
}

// ExpectedAddrsBalances is a map of base58 encoded addresses to the balances they should hold.
type ExpectedAddrsBalances map[string]map[devnetvm.Color]uint64

// RequireBalancesEqual asserts that all nodes report the balances as specified in balancesByAddress.
func RequireBalancesEqual(t *testing.T, nodes []*framework.Node, balancesByAddress map[string]map[devnetvm.Color]uint64) {
	for _, node := range nodes {
		for addrString, balances := range balancesByAddress {
			for color, balance := range balances {
				addr, err := devnetvm.AddressFromBase58EncodedString(addrString)
				require.NoErrorf(t, err, "invalid address string: %s", addrString)

				actualBalance := Balance(t, node, addr, color)
				require.Equalf(t, balance, actualBalance, "balance for color '%s' on address '%s' (node='%s') does not match, expected=%d, actual=%d", color, addr.Base58(), node, balance, actualBalance)
			}
		}
	}
}

// ExpectedState is an expected state.
// All fields are optional.
type ExpectedState struct {
	// The optional confirmation state to check against.
	ConfirmationState confirmation.State
	// The optional solid state to check against.
	Solid *bool
}

// True returns a pointer to a true bool.
func True() *bool {
	x := true
	return &x
}

// ExpectedTransaction defines the expected data of a transaction.
// All fields are optional.
type ExpectedTransaction struct {
	Inputs []*jsonmodels.Input
	// The optional outputs to check against.
	Outputs []*jsonmodels.Output
	// The optional unlock blocks to check against.
	UnlockBlocks []*jsonmodels.UnlockBlock
}

// RequireTransactionsEqual asserts that all nodes return the correct transactions as specified in transactionsByID.
func RequireTransactionsEqual(t *testing.T, nodes []*framework.Node, transactionsByID map[string]*ExpectedTransaction) {
	for _, node := range nodes {
		for txID, expTransaction := range transactionsByID {
			transaction, err := node.GetTransaction(txID)
			require.NoErrorf(t, err, "node%s, txID=%s, 'GetTransaction' failed", node, txID)

			if expTransaction != nil {
				if expTransaction.Inputs != nil {
					require.Equalf(t, expTransaction.Inputs, transaction.Inputs, "node=%s, txID=%s, inputs do not match", node, txID)
				}
				if expTransaction.Outputs != nil {
					require.Equalf(t, expTransaction.Outputs, transaction.Outputs, "node=%s, txID=%s, outputs do not match", node, txID)
				}
				if expTransaction.UnlockBlocks != nil {
					require.Equalf(t, expTransaction.UnlockBlocks, transaction.UnlockBlocks, "node=%s, txID=%s, signatures do not match", node, txID)
				}
			}
		}
	}
}

// ExpectedTxsStates is a map of base58 encoded transactionIDs to their ExpectedState(s).
type ExpectedTxsStates map[string]ExpectedState

// RequireConfirmationStateEqual asserts that all nodes have received the transaction and have correct expectedStates
// in waitFor time, periodically checking each tick.
func RequireConfirmationStateEqual(t *testing.T, nodes framework.Nodes, expectedStates ExpectedTxsStates, waitFor time.Duration, tick time.Duration) {
	condition := func() bool {
		for _, node := range nodes {
			for txID, expInclState := range expectedStates {
				_, err := node.GetTransaction(txID)
				// retry, when the transaction could not be found
				if errors.Is(err, client.ErrNotFound) {
					continue
				}
				require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransaction' failed", node, txID)

				// the confirmation state can change, so we should check all transactions every time
				stateEqual, confirmationState := txMetadataStateEqual(t, node, txID, expInclState)
				if !stateEqual {
					t.Logf("Current ConfirmationState for txId %s is %s on %s", txID, confirmationState, node.Name())
					return false
				}
				t.Logf("Current ConfirmationState for txId %s is %s on %s", txID, confirmationState, node.Name())

			}
		}
		return true
	}

	log.Printf("Waiting for %d transactions to reach the correct ConfirmationState...", len(expectedStates))
	require.Eventually(t, condition, waitFor, tick)
	log.Println("Waiting for ConfirmationState... done")
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(ctx context.Context, t *testing.T, n interface{ Shutdown(context.Context) error }) {
	log.Println("Shutting down network...")
	require.NoError(t, n.Shutdown(ctx))
	log.Println("Shutting down network... done")
}

// OutputIndex returns the index of the first output to address.
func OutputIndex(transaction *devnetvm.Transaction, address devnetvm.Address) int {
	for i, output := range transaction.Essence().Outputs() {
		if output.Address().Equals(address) {
			return i
		}
	}
	panic("invalid address")
}

func txMetadataStateEqual(t *testing.T, node *framework.Node, txID string, expInclState ExpectedState) (bool, confirmation.State) {
	metadata, err := node.GetTransactionMetadata(txID)
	require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransactionMetadata' failed")

	if (expInclState.ConfirmationState != confirmation.Undefined && metadata.ConfirmationState < expInclState.ConfirmationState) ||
		(expInclState.Solid != nil && *expInclState.Solid != metadata.Booked) {
		return false, metadata.ConfirmationState
	}
	return true, metadata.ConfirmationState
}

// AcceptedOnAllPeers checks if the blk is accepted on all supplied peers.
func AcceptedOnAllPeers(blockID string, peers []*framework.Node) bool {
	for _, peer := range peers {
		metadata, err := peer.GetBlockMetadata(blockID)
		if err != nil {
			return false
		}
		if !metadata.M.Accepted {
			return false
		}
	}
	return true
}

// TryAcceptBlock tries to accept the block on all the peers provided within the time limit provided.
func TryAcceptBlock(t *testing.T, peers []*framework.Node, blockID string, waitFor time.Duration, tick time.Duration) {
	log.Printf("waiting for blk %s to become accepted...", blockID)
	defer log.Printf("waiting for blk %s to become accepted... done", blockID)

	timer := time.NewTimer(waitFor)
	defer timer.Stop()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			log.Printf("failed to confirm block %s within the time limit", blockID)
			t.FailNow()
		case <-ticker.C:
			// Issue a new block on each peer to make block confirmed.
			for i, peer := range peers {
				id, _ := SendDataBlock(t, peer, []byte("test"), i)
				log.Printf("send block %s on node %s", id, peer.ID())
			}

			if AcceptedOnAllPeers(blockID, peers) {
				log.Printf("block %s is accepted on all peers", blockID)
				return
			}
		}
	}
}
