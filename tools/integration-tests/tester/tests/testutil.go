package tests

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
)

var faucetPoWDifficulty = framework.PeerConfig().Faucet.PowDifficulty

const (
	// Timeout denotes the default condition polling timout duration.
	Timeout = 1 * time.Minute
	// Tick denotes the default condition polling tick time.
	Tick = 500 * time.Millisecond

	shutdownGraceTime = time.Minute
)

// EqualSnapshotDetails defines info for equally distributed consensus mana.
var EqualSnapshotDetails = framework.SnapshotInfo{
	FilePath:           "/assets/dynamic_snapshots/equal_snapshot.bin",
	MasterSeed:         "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E", // FZ6xmPZX
	GenesisTokenAmount: 2_500_000_000_000_000,
	PeersSeedBase58: []string{
		"GtKSdqanb4mokUBjAf9JZmsSqWzWjzzw57mRR56LjfBL", // H6jzPnLbjsh
		"CmFVE14Yh9rqn2FrXD8s7ybRoRN5mUnqQxLAuD5HF2em", // JHxvcap7xhv
		"DuJuWE3hisFrFK1HmrXkd9FSsNNWbw58JcQnKdBn6TdN", // 7rRpyEGU7Sf
	},
	PeersAmountsPledged: []uint64{2_500_000_000_000_000, 2_500_000_000_000_000, 2_500_000_000_000_000},
}

// ConsensusSnapshotDetails defines info for consensus integration test snapshot
var ConsensusSnapshotDetails = framework.SnapshotInfo{
	FilePath: "/assets/dynamic_snapshots/consensus_snapshot.bin",
	// node ID: 2GtxMQD9
	MasterSeed:         "EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP",
	GenesisTokenAmount: 800_000, // pledged to peer master
	// peer IDs: jnaC6ZyWuw, iNvPFvkfSDp
	PeersSeedBase58: []string{
		"Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8",
		"HUH4rmxUxMZBBtHJ4QM5Ts6s8DP3HnFpChejntnCxto2",
	},
	PeersAmountsPledged: []uint64{1_600_000, 800_000},
}

// GetIdentSeed returns decoded seed bytes for the supplied SnapshotInfo and peer index
func GetIdentSeed(t *testing.T, snapshotInfo framework.SnapshotInfo, peerIndex int) []byte {
	seedBytes, err := base58.Decode(snapshotInfo.PeersSeedBase58[peerIndex])
	require.NoError(t, err)
	return seedBytes
}

// CommonSnapshotConfigFunc returns a peer configuration altering function that uses the specified Snapshot information for all peers.
// If a cfgFunc is provided, further manipulation of the base config for every peer is possible.
func CommonSnapshotConfigFunc(t *testing.T, snaphotInfo framework.SnapshotInfo, cfgFunc ...framework.CfgAlterFunc) framework.CfgAlterFunc {
	return func(peerIndex int, isPeerMaster bool, conf config.GoShimmer) config.GoShimmer {
		conf.BlockLayer.Snapshot.File = snaphotInfo.FilePath
		if isPeerMaster {
			seedBytes, err := base58.Decode(snaphotInfo.MasterSeed)
			require.NoError(t, err)
			conf.Seed = seedBytes
			return conf
		}

		require.Lessf(t, peerIndex, len(snaphotInfo.PeersSeedBase58), "index=%d out of range for peerSeeds=%d", peerIndex, len(snaphotInfo.PeersSeedBase58))
		conf.Seed = GetIdentSeed(t, snaphotInfo, peerIndex)

		if len(cfgFunc) > 0 {
			conf = cfgFunc[0](peerIndex, isPeerMaster, conf)
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
	RequireBlocksAvailable(t, []*framework.Node{node}, map[string]DataBlockSent{sent.id: sent}, Timeout, Tick, confirmation.Pending)

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
	id, err := node.Data(data)
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
func SendDataBlocksWithDelay(t *testing.T, peers []*framework.Node, numBlocks int, delay time.Duration) (result map[string]DataBlockSent) {
	result = make(map[string]DataBlockSent, numBlocks)
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
		outputColor = blake2b.Sum256(mintOutput.ID().Bytes())
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
func RequireBlocksAvailable(t *testing.T, nodes []*framework.Node, blockIDs map[string]DataBlockSent, waitFor time.Duration, tick time.Duration, confirmationState ...confirmation.State) {
	missing := make(map[identity.ID]map[string]struct{}, len(nodes))
	for _, node := range nodes {
		missing[node.ID()] = make(map[string]struct{}, len(blockIDs))
		for blockID := range blockIDs {
			missing[node.ID()][blockID] = struct{}{}
		}
	}

	condition := func() bool {
		for _, node := range nodes {
			nodeMissing := missing[node.ID()]
			for blockID := range nodeMissing {
				blk, err := node.GetBlockMetadata(blockID)
				// retry, when the block could not be found
				if errors.Is(err, client.ErrNotFound) {
					log.Printf("node=%s, blockID=%s; block not found", node, blockID)
					continue
				}
				// retry, if the block has not yet reached the specified ConfirmationState
				if len(confirmationState) > 0 {
					if blk.ConfirmationState < confirmationState[0] {
						log.Printf("node=%s, blockID=%s, expected ConfirmationState=%s, actual ConfirmationState=%s; ConfirmationState not reached", node, blockID, confirmationState[0], blk.ConfirmationState)
						continue
					}
				}

				require.NoErrorf(t, err, "node=%s, blockID=%s, 'GetBlockMetadata' failed", node, blockID)
				require.Equal(t, blockID, blk.ID)
				delete(nodeMissing, blockID)
				if len(nodeMissing) == 0 {
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

// RequireBlocksEqual asserts that all nodes return the correct data blocks as specified in blocksByID.
func RequireBlocksEqual(t *testing.T, nodes []*framework.Node, blocksByID map[string]DataBlockSent, waitFor time.Duration, tick time.Duration) {
	condition := func() bool {
		for _, node := range nodes {
			for blockID := range blocksByID {
				resp, err := node.GetBlock(blockID)
				require.NoErrorf(t, err, "node=%s, blockID=%s, 'GetBlock' failed", node, blockID)
				require.Equal(t, resp.ID, blockID)

				respMetadata, err := node.GetBlockMetadata(blockID)
				require.NoErrorf(t, err, "node=%s, blockID=%s, 'GetBlockMetadata' failed", node, blockID)
				require.Equal(t, respMetadata.ID, blockID)

				// check for general information
				blkSent := blocksByID[blockID]

				require.Equalf(t, blkSent.issuerPublicKey, resp.IssuerPublicKey, "blockID=%s, issuer=%s not correct issuer in %s.", blkSent.id, blkSent.issuerPublicKey, node)
				if blkSent.data != nil {
					require.Equalf(t, blkSent.data, resp.Payload, "blockID=%s, issuer=%s data not equal in %s.", blkSent.id, blkSent.issuerPublicKey, node)
				}

				if !respMetadata.Solid {
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
				require.Equalf(t, balance, Balance(t, node, addr, color),
					"balance for color '%s' on address '%s' (node='%s') does not match", color, addr.Base58(), node)
			}
		}
	}
}

// RequireNoUnspentOutputs asserts that on all node the given addresses do not have any unspent outputs.
func RequireNoUnspentOutputs(t *testing.T, nodes []*framework.Node, addresses ...devnetvm.Address) {
	for _, node := range nodes {
		for _, addr := range addresses {
			unspent := AddressUnspentOutputs(t, node, addr, 1)
			require.Empty(t, unspent, "address %s should not have any UTXOs", addr)
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

// False returns a pointer to a false bool.
func False() *bool {
	x := false
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
					t.Logf("Current ConfirmationState for txId %s is %s", txID, confirmationState)
					return false
				}
				t.Logf("Current ConfirmationState for txId %s is %s", txID, confirmationState)

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
func AcceptedOnAllPeers(blkID string, peers []*framework.Node) bool {
	for _, peer := range peers {
		metadata, err := peer.GetBlockMetadata(blkID)
		if err != nil {
			return false
		}
		if !metadata.ConfirmationState.IsAccepted() {
			return false
		}
	}
	return true
}

// TryConfirmBlock tries to confirm the block on all the peers provided within the time limit provided.
func TryConfirmBlock(t *testing.T, peers []*framework.Node, blkID string, waitFor time.Duration, tick time.Duration) {
	log.Printf("waiting for blk %s to become confirmed...", blkID)
	defer log.Printf("waiting for blk %s to become confirmed... done", blkID)

	timer := time.NewTimer(waitFor)
	defer timer.Stop()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			log.Printf("failed to confirm blk %s within the time limit", blkID)
			t.FailNow()
		case <-ticker.C:
			// Issue a new block on each peer to make blk confirmed.
			for i, peer := range peers {
				id, _ := SendDataBlock(t, peer, []byte("test"), i)
				log.Printf("send block %s on node %s", id, peer.ID())
			}

			if AcceptedOnAllPeers(blkID, peers) {
				log.Printf("blk %s is confirmed on all peers", blkID)
				return
			}
		}
	}
}
