package tests

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"

	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
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

	FaucetFundingOutputsAddrStart = 127
)

// EqualSnapshotDetails defines info for equally distributed consensus mana.
var EqualSnapshotDetails = framework.SnapshotInfo{
	FilePath: "/assets/dynamic_snapshots/equal_snapshot.bin",
	// node ID: dAnF7pQ6k7a
	MasterSeed:         "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E",
	GenesisTokenAmount: 2_500_000_000_000_000, // pledged to peer master
	// peer IDs: H6jzPnLbjsh, JHxvcap7xhv, 7rRpyEGU7Sf
	PeersSeedBase58: []string{
		"GtKSdqanb4mokUBjAf9JZmsSqWzWjzzw57mRR56LjfBL",
		"CmFVE14Yh9rqn2FrXD8s7ybRoRN5mUnqQxLAuD5HF2em",
		"DuJuWE3hisFrFK1HmrXkd9FSsNNWbw58JcQnKdBn6TdN",
	},
	PeersAmountsPledged: []uint64{2_500_000_000_000_000, 2_500_000_000_000_000, 2_500_000_000_000_000},
}

// ConsensusSnapshotDetails defines info for consensus integration test snapshot, messages approved with gof threshold set up to 75%
var ConsensusSnapshotDetails = framework.SnapshotInfo{
	FilePath: "/assets/dynamic_snapshots/consensus_snapshot.bin",
	// node ID: 4AeXyZ26e4G
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
		conf.MessageLayer.Snapshot.File = snaphotInfo.FilePath
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

// DataMessageSent defines a struct to identify from which issuer a data message was sent.
type DataMessageSent struct {
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

// AwaitInitialFaucetOutputsPrepared waits until the initial outputs are prepared by the faucet.
func AwaitInitialFaucetOutputsPrepared(t *testing.T, faucet *framework.Node, peers []*framework.Node) {
	supplyOutputsCount := faucet.Config().SupplyOutputsCount
	splittingMultiplier := faucet.Config().SplittingMultiplier
	lastFundingOutputAddress := supplyOutputsCount*splittingMultiplier + FaucetFundingOutputsAddrStart - 1
	addrToCheck := faucet.Address(lastFundingOutputAddress).Base58()

	confirmed := make(map[int]types.Empty)
	require.Eventually(t, func() bool {
		if len(confirmed) == supplyOutputsCount*splittingMultiplier {
			return true
		}
		// wait for confirmation of each fundingOutput
		for fundingIndex := FaucetFundingOutputsAddrStart; fundingIndex <= lastFundingOutputAddress; fundingIndex++ {
			if _, ok := confirmed[fundingIndex]; !ok {
				resp, err := faucet.PostAddressUnspentOutputs([]string{addrToCheck})
				require.NoError(t, err)
				if len(resp.UnspentOutputs[0].Outputs) != 0 {
					if resp.UnspentOutputs[0].Outputs[0].GradeOfFinality == gof.High {
						confirmed[fundingIndex] = types.Void
					}
				}
			}
		}
		return false
	}, time.Minute, Tick)
	// give the faucet time to save the latest confirmed output
	time.Sleep(3 * time.Second)
}

// AddressUnspentOutputs returns the unspent outputs on address.
func AddressUnspentOutputs(t *testing.T, node *framework.Node, address ledgerstate.Address, numOfExpectedOuts int) []jsonmodels.WalletOutput {
	resp, err := node.PostAddressUnspentOutputs([]string{address.Base58()})
	require.NoErrorf(t, err, "node=%s, address=%s, PostAddressUnspentOutputs failed", node, address.Base58())
	require.Lenf(t, resp.UnspentOutputs, numOfExpectedOuts, "invalid response")
	require.Equalf(t, address.Base58(), resp.UnspentOutputs[0].Address.Base58, "invalid response")

	return resp.UnspentOutputs[0].Outputs
}

// Balance returns the total balance of color at address.
func Balance(t *testing.T, node *framework.Node, address ledgerstate.Address, color ledgerstate.Color) uint64 {
	unspentOutputs := AddressUnspentOutputs(t, node, address, 1)

	var sum uint64
	for _, output := range unspentOutputs {
		out, err := output.Output.ToLedgerstateOutput()
		require.NoError(t, err)
		balance, _ := out.Balances().Get(color)
		sum += balance
	}
	return sum
}

// SendFaucetRequest sends a data message on a given peer and returns the id and a DataMessageSent struct. By default,
// it pledges mana to the peer making the request.
func SendFaucetRequest(t *testing.T, node *framework.Node, addr ledgerstate.Address, manaPledgeIDs ...string) (string, DataMessageSent) {
	nodeID := base58.Encode(node.ID().Bytes())
	aManaPledgeID, cManaPledgeID := nodeID, nodeID
	if len(manaPledgeIDs) > 1 {
		aManaPledgeID, cManaPledgeID = manaPledgeIDs[0], manaPledgeIDs[1]
	}

	resp, err := node.SendFaucetRequest(addr.Base58(), faucetPoWDifficulty, aManaPledgeID, cManaPledgeID)
	require.NoErrorf(t, err, "node=%s, address=%s, SendFaucetRequest failed", node, addr.Base58())

	sent := DataMessageSent{
		id:              resp.ID,
		data:            nil,
		issuerPublicKey: node.Identity.PublicKey().String(),
	}

	// Make sure the message is available on the peer itself and has gof.Low.
	RequireMessagesAvailable(t, []*framework.Node{node}, map[string]DataMessageSent{sent.id: sent}, Timeout, Tick, gof.Low)

	return resp.ID, sent
}

// region CreateTransaction from outputs //////////////////////////////

// CreateTransactionFromOutputs takes the given utxos inputs and create a transaction that spreads the total input balance
// across the targetAddresses. In order to correctly sign we have a keyPair map that maps a given address to its public key.
// Access and Consensus Mana is pledged to the node we specify.
func CreateTransactionFromOutputs(t *testing.T, manaPledgeID identity.ID, targetAddresses []ledgerstate.Address, keyPairs map[string]*ed25519.KeyPair, utxos ...ledgerstate.Output) *ledgerstate.Transaction {
	// Create Inputs from utxos
	inputs := ledgerstate.Inputs{}
	balances := map[ledgerstate.Color]uint64{}
	for _, output := range utxos {
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			balances[color] += balance
			return true
		})
		inputs = append(inputs, output.Input())
	}

	// create outputs for each target address
	numberOfOutputs := len(targetAddresses)
	outputs := make(ledgerstate.Outputs, numberOfOutputs)
	for i := 0; i < numberOfOutputs; i++ {
		outBalances := map[ledgerstate.Color]uint64{}
		for color, balance := range balances {
			// divide by number of outputs to spread funds evenly
			outBalances[color] = balance / uint64(numberOfOutputs)
			if i == numberOfOutputs-1 {
				// on the last iteration add the remainder so all funds are consumed
				outBalances[color] += balance % uint64(numberOfOutputs)
			}
		}
		outputs[i] = ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(outBalances), targetAddresses[i])
	}

	// create tx essence
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), manaPledgeID,
		manaPledgeID, inputs, ledgerstate.NewOutputs(outputs...))

	// create signatures
	unlockBlocks := make([]ledgerstate.UnlockBlock, len(inputs))

	for i := 0; i < len(inputs); i++ {
		addressKey := utxos[i].Address().String()
		keyPair := keyPairs[addressKey]
		require.NotNilf(t, keyPair, "missing key pair for address %s", addressKey)
		sig := ledgerstate.NewED25519Signature(keyPair.PublicKey, keyPair.PrivateKey.Sign(txEssence.Bytes()))
		unlockBlocks[i] = ledgerstate.NewSignatureUnlockBlock(sig)
	}

	return ledgerstate.NewTransaction(txEssence, unlockBlocks)
}

// endregion

// SendDataMessage sends a data message on a given peer and returns the id and a DataMessageSent struct.
func SendDataMessage(t *testing.T, node *framework.Node, data []byte, number int) (string, DataMessageSent) {
	id, err := node.Data(data)
	require.NoErrorf(t, err, "node=%s, 'Data' failed", node)

	sent := DataMessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewGenericDataPayload(data).Bytes(),
		issuerPublicKey: node.Identity.PublicKey().String(),
	}
	return id, sent
}

// SendDataMessages sends a total of numMessages data messages and saves the sent message to a map.
// It chooses the peers to send the messages from in a round-robin fashion.
func SendDataMessages(t *testing.T, peers []*framework.Node, numMessages int, idsMap ...map[string]DataMessageSent) map[string]DataMessageSent {
	var result map[string]DataMessageSent
	if len(idsMap) > 0 {
		result = idsMap[0]
	} else {
		result = make(map[string]DataMessageSent, numMessages)
	}

	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test: %d", i))

		id, sent := SendDataMessage(t, peers[i%len(peers)], data, i)
		result[id] = sent
	}
	return result
}

// SendDataMessagesWithDelay sends a total of numMessages data messages, each after a delay interval, and saves the sent message to a map.
// It chooses the peers to send the messages from in a round-robin fashion.
func SendDataMessagesWithDelay(t *testing.T, peers []*framework.Node, numMessages int, delay time.Duration) (result map[string]DataMessageSent) {
	result = make(map[string]DataMessageSent, numMessages)
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test: %d", i))

		id, sent := SendDataMessage(t, peers[i%len(peers)], data, i)
		result[id] = sent
		<-ticker.C
	}

	return
}

// SendTransaction sends a transaction of value and color. It returns the transactionID and the error return by PostTransaction.
// If addrBalance is given the balance mutation are added to that map.
func SendTransaction(t *testing.T, from *framework.Node, to *framework.Node, color ledgerstate.Color, value uint64, txConfig TransactionConfig, addrBalance ...map[string]map[ledgerstate.Color]uint64) (string, error) {
	inputAddr := from.Seed.Address(txConfig.FromAddressIndex).Address()
	outputAddr := to.Seed.Address(txConfig.ToAddressIndex).Address()

	unspentOutputs := AddressUnspentOutputs(t, from, inputAddr, 1)
	require.NotEmptyf(t, unspentOutputs, "address=%s, no unspent outputs", inputAddr.Base58())

	inputColor := color
	if color == ledgerstate.ColorMint {
		inputColor = ledgerstate.ColorIOTA
	}
	balance := Balance(t, from, inputAddr, inputColor)
	require.GreaterOrEqualf(t, balance, value, "address=%s, insufficient balance", inputAddr.Base58())

	out, err := unspentOutputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)

	input := ledgerstate.NewUTXOInput(out.ID())

	var outputs ledgerstate.Outputs
	output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		color: value,
	}), outputAddr)
	outputs = append(outputs, output)

	// handle remainder address
	if balance > value {
		output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			inputColor: balance - value,
		}), inputAddr)
		outputs = append(outputs, output)
	}

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), txConfig.AccessManaPledgeID, txConfig.ConsensusManaPledgeID, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(outputs...))
	sig := ledgerstate.NewED25519Signature(from.KeyPair(txConfig.FromAddressIndex).PublicKey, from.KeyPair(txConfig.FromAddressIndex).PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	txn := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})

	outputColor := color
	if color == ledgerstate.ColorMint {
		mintOutput := txn.Essence().Outputs()[OutputIndex(txn, outputAddr)]
		outputColor = blake2b.Sum256(mintOutput.ID().Bytes())
	}

	// send transaction
	resp, err := from.PostTransaction(txn.Bytes())
	if err != nil {
		return "", err
	}

	if len(addrBalance) > 0 {
		if addrBalance[0][inputAddr.Base58()] == nil {
			addrBalance[0][inputAddr.Base58()] = make(map[ledgerstate.Color]uint64)
		}
		addrBalance[0][inputAddr.Base58()][inputColor] -= value
		if addrBalance[0][outputAddr.Base58()] == nil {
			addrBalance[0][outputAddr.Base58()] = make(map[ledgerstate.Color]uint64)
		}
		addrBalance[0][outputAddr.Base58()][outputColor] += value
	}
	return resp.TransactionID, nil
}

// RequireMessagesAvailable asserts that all nodes have received MessageIDs in waitFor time, periodically checking each tick.
// Optionally, a GradeOfFinality can be specified, which then requires the messages to reach this GradeOfFinality.
func RequireMessagesAvailable(t *testing.T, nodes []*framework.Node, messageIDs map[string]DataMessageSent, waitFor time.Duration, tick time.Duration, gradeOfFinality ...gof.GradeOfFinality) {
	missing := make(map[identity.ID]map[string]struct{}, len(nodes))
	for _, node := range nodes {
		missing[node.ID()] = make(map[string]struct{}, len(messageIDs))
		for messageID := range messageIDs {
			missing[node.ID()][messageID] = struct{}{}
		}
	}

	condition := func() bool {
		for _, node := range nodes {
			nodeMissing := missing[node.ID()]
			for messageID := range nodeMissing {
				msg, err := node.GetMessageMetadata(messageID)
				// retry, when the message could not be found
				if errors.Is(err, client.ErrNotFound) {
					log.Printf("node=%s, messageID=%s; message not found", node, messageID)
					continue
				}
				// retry, if the message has not yet reached the specified GoF
				if len(gradeOfFinality) > 0 {
					if msg.GradeOfFinality < gradeOfFinality[0] {
						log.Printf("node=%s, messageID=%s, expected GoF=%s, actual GoF=%s; GoF not reached", node, messageID, gradeOfFinality[0], msg.GradeOfFinality)
						continue
					}
				}

				require.NoErrorf(t, err, "node=%s, messageID=%s, 'GetMessageMetadata' failed", node, messageID)
				require.Equal(t, messageID, msg.ID)
				delete(nodeMissing, messageID)
				if len(nodeMissing) == 0 {
					delete(missing, node.ID())
				}
			}
		}
		return len(missing) == 0
	}

	log.Printf("Waiting for %d messages to become available...", len(messageIDs))
	require.Eventuallyf(t, condition, waitFor, tick,
		"%d out of %d nodes did not receive all messages", len(missing), len(nodes))
	log.Println("Waiting for message... done")
}

// RequireMessagesEqual asserts that all nodes return the correct data messages as specified in messagesByID.
func RequireMessagesEqual(t *testing.T, nodes []*framework.Node, messagesByID map[string]DataMessageSent) {
	for _, node := range nodes {
		for messageID := range messagesByID {
			resp, err := node.GetMessage(messageID)
			require.NoErrorf(t, err, "node=%s, messageID=%s, 'GetMessage' failed", node, messageID)
			require.Equal(t, resp.ID, messageID)

			respMetadata, err := node.GetMessageMetadata(messageID)
			require.NoErrorf(t, err, "node=%s, messageID=%s, 'GetMessageMetadata' failed", node, messageID)
			require.Equal(t, respMetadata.ID, messageID)

			// check for general information
			msgSent := messagesByID[messageID]

			require.Equalf(t, msgSent.issuerPublicKey, resp.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, node)
			if msgSent.data != nil {
				require.Equalf(t, msgSent.data, resp.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, node)
			}
			require.Truef(t, respMetadata.Solid, "messageID=%s, issuer=%s not solid in %s", msgSent.id, msgSent.issuerPublicKey, node)
		}
	}
}

// ExpectedAddrsBalances is a map of base58 encoded addresses to the balances they should hold.
type ExpectedAddrsBalances map[string]map[ledgerstate.Color]uint64

// RequireBalancesEqual asserts that all nodes report the balances as specified in balancesByAddress.
func RequireBalancesEqual(t *testing.T, nodes []*framework.Node, balancesByAddress map[string]map[ledgerstate.Color]uint64) {
	for _, node := range nodes {
		for addrString, balances := range balancesByAddress {
			for color, balance := range balances {
				addr, err := ledgerstate.AddressFromBase58EncodedString(addrString)
				require.NoErrorf(t, err, "invalid address string: %s", addrString)
				require.Equalf(t, balance, Balance(t, node, addr, color),
					"balance for color '%s' on address '%s' (node='%s') does not match", color, addr.Base58(), node)
			}
		}
	}
}

// RequireNoUnspentOutputs asserts that on all node the given addresses do not have any unspent outputs.
func RequireNoUnspentOutputs(t *testing.T, nodes []*framework.Node, addresses ...ledgerstate.Address) {
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
	// The optional grade of finality state to check against.
	GradeOfFinality *gof.GradeOfFinality
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

// GoFPointer returns a pointer to the given grade of finality value.
func GoFPointer(gradeOfFinality gof.GradeOfFinality) *gof.GradeOfFinality {
	return &gradeOfFinality
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

// RequireGradeOfFinalityEqual asserts that all nodes have received the transaction and have correct expectedStates
// in waitFor time, periodically checking each tick.
func RequireGradeOfFinalityEqual(t *testing.T, nodes framework.Nodes, expectedStates ExpectedTxsStates, waitFor time.Duration, tick time.Duration) {
	condition := func() bool {
		for _, node := range nodes {
			for txID, expInclState := range expectedStates {
				_, err := node.GetTransaction(txID)
				// retry, when the transaction could not be found
				if errors.Is(err, client.ErrNotFound) {
					continue
				}
				require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransaction' failed", node, txID)

				// the grade of finality can change, so we should check all transactions every time
				stateEqual, gof := txMetadataStateEqual(t, node, txID, expInclState)
				if !stateEqual {
					t.Logf("Current grade of finality for txId %s is %d", txID, gof)
					return false
				}
				t.Logf("Current grade of finality for txId %s is %d", txID, gof)

			}
		}
		return true
	}

	log.Printf("Waiting for %d transactions to reach the correct grade of finality...", len(expectedStates))
	require.Eventually(t, condition, waitFor, tick)
	log.Println("Waiting for grade of finality... done")
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(ctx context.Context, t *testing.T, n interface{ Shutdown(context.Context) error }) {
	log.Println("Shutting down network...")
	require.NoError(t, n.Shutdown(ctx))
	log.Println("Shutting down network... done")
}

// OutputIndex returns the index of the first output to address.
func OutputIndex(transaction *ledgerstate.Transaction, address ledgerstate.Address) int {
	for i, output := range transaction.Essence().Outputs() {
		if output.Address().Equals(address) {
			return i
		}
	}
	panic("invalid address")
}

func txMetadataStateEqual(t *testing.T, node *framework.Node, txID string, expInclState ExpectedState) (bool, gof.GradeOfFinality) {
	metadata, err := node.GetTransactionMetadata(txID)
	require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransactionMetadata' failed")

	if (expInclState.GradeOfFinality != nil && *expInclState.GradeOfFinality != metadata.GradeOfFinality) ||
		(expInclState.Solid != nil && *expInclState.Solid != metadata.Solid) {
		return false, metadata.GradeOfFinality
	}
	return true, metadata.GradeOfFinality
}

// ConfirmedOnAllPeers checks if the msg is confirmed on all supplied peers.
func ConfirmedOnAllPeers(msgID string, peers []*framework.Node) bool {
	for _, peer := range peers {
		metadata, err := peer.GetMessageMetadata(msgID)
		if err != nil {
			return false
		}
		if metadata.GradeOfFinality != gof.High {
			return false
		}
	}
	return true
}

// TryConfirmMessage tries to confirm the message on all the peers provided within the time limit provided.
func TryConfirmMessage(t *testing.T, n *framework.Network, requiredPeers []*framework.Node, msgID string, waitFor time.Duration, tick time.Duration) {
	var peers []*framework.Node
	for _, peer := range n.Peers() {
		if _, err := peer.GetMessage(msgID); err == nil {
			peers = append(peers, peer)
		}
	}
	if len(peers) == 0 {
		log.Println("msg ", msgID, "is not found on any node")
		t.FailNow()
	}

	var i int
	timer := time.NewTimer(waitFor)
	defer timer.Stop()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			log.Println("timeout")
			t.FailNow()
		case <-ticker.C:
			if ConfirmedOnAllPeers(msgID, requiredPeers) {
				log.Println("msg is confirmed on all required peers")
				return
			}
		default:
			// sort nodes by cmana before issuing?
			node := peers[i%len(peers)]
			if _, err := node.Data([]byte("test")); err != nil {
				log.Println("send message on node: ", node.ID().String())
			}
			i++
		}
	}
}

// IsBranchConfirmedOnAllPeers returns true if the branch is confirmed on all supplied nodes.
func IsBranchConfirmedOnAllPeers(branchID string, peers []*framework.Node) bool {
	for _, peer := range peers {
		branch, err := peer.GetBranch(branchID)
		if err != nil {
			return false
		}
		if branch.GradeOfFinality != gof.High {
			return false
		}
	}
	return true
}

func findAttachmentMsg(peer *framework.Node, branchID string) (tip *jsonmodels.Message, err error) {
	branch, err := peer.GetBranch(branchID)
	if err != nil {
		return
	}
	attachments, err := peer.GetTransactionAttachments(branchID)
	if err != nil {
		return
	}
	approversWalker := walker.New(false)
	for _, msgID := range attachments.MessageIDs {
		approversWalker.Push(msgID)
	}
	for approversWalker.HasNext() {
		tip, err = peer.GetMessage(approversWalker.Next().(string))
		if err != nil {
			return
		}
		for _, approverMsgID := range tip.StrongApprovers {
			var (
				approverMsg *jsonmodels.Message
				metadata    *jsonmodels.MessageMetadata
			)
			if approverMsg, err = peer.GetMessage(approverMsgID); err == nil {
				if metadata, err = peer.GetMessageMetadata(approverMsgID); err == nil {
					if metadata.BranchIDs[0] == branch.ID {
						approversWalker.Push(approverMsgID)
						continue
					}

					approverLikes := false
					for _, like := range approverMsg.ShallowLikeParents {
						if like == tip.ID {
							approverLikes = true
							break
						}
					}
					if approverLikes {
						approversWalker.Push(approverMsgID)
						continue
					}
				}
			}
		}
	}
	return
}

// TryConfirmBranch tries to confirm the given branch in the duration specified.
func TryConfirmBranch(t *testing.T, n *framework.Network, requiredPeers []*framework.Node, branchID string, waitFor time.Duration, tick time.Duration) {
	if branchID == ledgerstate.MasterBranchID.Base58() {
		return
	}

	// check that the branch exists in the network and fail fast if it does not
	exists := false
	for _, peer := range n.Peers() {
		if _, err := peer.GetBranch(branchID); err == nil {
			exists = true
			break
		}
	}
	require.True(t, exists, "branch does not exists on any node")

	// get tip to attach
	tip, err := findAttachmentMsg(requiredPeers[0], branchID)
	require.NoError(t, err)

	// issue messages on top of tip.
	timer := time.NewTimer(waitFor)
	defer timer.Stop()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	var i int
	peers := n.Peers()

	for {
		select {
		case <-timer.C:
			require.FailNow(t, "timeout")
		case <-ticker.C:
			if IsBranchConfirmedOnAllPeers(branchID, requiredPeers) {
				return
			}
		default:
			var parentMessageIDs []jsonmodels.ParentMessageIDs
			if ID, err := tangle.NewMessageID(tip.ID); err == nil {
				if tip.PayloadType == ledgerstate.TransactionType.String() {
					parentMessageIDs = append(parentMessageIDs, jsonmodels.ParentMessageIDs{
						Type:       uint8(tangle.ShallowLikeParentType),
						MessageIDs: []string{ID.Base58()},
					})
				}
				parentMessageIDs = append(parentMessageIDs, jsonmodels.ParentMessageIDs{
					Type:       uint8(tangle.StrongParentType),
					MessageIDs: []string{ID.Base58()},
				})
			}

			require.NoError(t, err)
			tipMsgID, err := peers[i%len(peers)].SendMessage(&jsonmodels.SendMessageRequest{
				Payload:          payload.NewGenericDataPayload([]byte("test")).Bytes(),
				ParentMessageIDs: parentMessageIDs,
			})
			require.NoError(t, err)
			tip, err = peers[i%len(peers)].GetMessage(tipMsgID)
			require.NoError(t, err)
			time.Sleep(500 * time.Millisecond)
			i++
		}
	}
}
