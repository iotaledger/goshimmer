package tests

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
)

var faucetPoWDifficulty = framework.PeerConfig.Faucet.PowDifficulty

const Tick = 500 * time.Millisecond

// DataMessageSent defines a struct to identify from which issuer a data message was sent.
type DataMessageSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

type Shutdowner interface {
	Shutdown(context.Context) error
}

// TransactionConfig defines the configuration for a transaction.
type TransactionConfig struct {
	FromAddressIndex      uint64
	ToAddressIndex        uint64
	AccessManaPledgeID    identity.ID
	ConsensusManaPledgeID identity.ID
}

// SendDataMessagesOnRandomPeer sends data messages on a random peer and saves the sent message to a map.
func SendDataMessagesOnRandomPeer(t *testing.T, peers []*framework.Node, numMessages int, idsMap ...map[string]DataMessageSent) map[string]DataMessageSent {
	var ids map[string]DataMessageSent
	if len(idsMap) > 0 {
		ids = idsMap[0]
	} else {
		ids = make(map[string]DataMessageSent, numMessages)
	}

	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test: %d", i))

		peer := peers[rand.Intn(len(peers))]
		id, sent := SendDataMessage(t, peer, data, i)

		ids[id] = sent
	}

	return ids
}

// SendDataMessage sends a data message on a given peer and returns the id and a DataMessageSent struct.
func SendDataMessage(t *testing.T, node *framework.Node, data []byte, number int) (string, DataMessageSent) {
	id, err := node.Data(data)
	require.NoErrorf(t, err, "node=%s, Data failed", node)

	sent := DataMessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewGenericDataPayload(data).Bytes(),
		issuerPublicKey: node.Identity.PublicKey().String(),
	}
	return id, sent
}

// SendFaucetRequest sends a data message on a given peer and returns the id and a DataMessageSent struct. By default,
// it pledges mana to the peer making the request.
func SendFaucetRequest(t *testing.T, peer *framework.Node, addr ledgerstate.Address, manaPledgeIDs ...string) (string, DataMessageSent) {
	peerID := base58.Encode(peer.ID().Bytes())
	aManaPledgeID, cManaPledgeID := peerID, peerID
	if len(manaPledgeIDs) > 1 {
		aManaPledgeID, cManaPledgeID = manaPledgeIDs[0], manaPledgeIDs[1]
	}

	resp, err := peer.SendFaucetRequest(addr.Base58(), faucetPoWDifficulty, aManaPledgeID, cManaPledgeID)
	require.NoErrorf(t, err, "Could not send faucet request on %s", peer.String())

	sent := DataMessageSent{
		id:              resp.ID,
		data:            nil,
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return resp.ID, sent
}

// RequireMessagesAvailable asserts that all nodes have received MessageIDs in waitFor time, periodically checking each tick.
func RequireMessagesAvailable(t *testing.T, nodes []*framework.Node, messageIDs map[string]DataMessageSent, waitFor time.Duration, tick time.Duration) {
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
				msg, err := node.GetMessage(messageID)
				// retry, when the message could not be found
				if errors.Is(err, client.ErrNotFound) {
					continue
				}
				require.NoErrorf(t, err, "GetMessage(%s) failed for node %s", messageID, node)
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

func AssertMessagesEqual(t *testing.T, nodes []*framework.Node, messageIDs map[string]DataMessageSent) {
	log.Printf("Validating %d messages...", len(messageIDs))
	for _, node := range nodes {
		for messageID := range messageIDs {
			resp, err := node.GetMessage(messageID)
			require.NoErrorf(t, err, "messageID=%s, GetMessage failed for %s", messageID, node)
			require.Equal(t, resp.ID, messageID)

			respMetadata, err := node.GetMessageMetadata(messageID)
			require.NoErrorf(t, err, "messageID=%s, GetMessageMetadata failed for %s", messageID, node)
			require.Equal(t, respMetadata.ID, messageID)

			// check for general information
			msgSent := messageIDs[messageID]

			assert.Equalf(t, msgSent.issuerPublicKey, resp.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, node)
			if msgSent.data != nil {
				assert.Equalf(t, msgSent.data, resp.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, node)
			}
			assert.Truef(t, respMetadata.Solid, "messageID=%s, issuer=%s not solid in %s", msgSent.id, msgSent.issuerPublicKey, node)
		}
	}
	log.Println("Validating messages... done")
}

func SendValue(t *testing.T, from *framework.Node, to *framework.Node, color ledgerstate.Color, value uint64, txConfig TransactionConfig, addrBalance ...map[string]map[ledgerstate.Color]uint64) (string, error) {
	inputAddr := from.Seed.Address(txConfig.FromAddressIndex).Address()
	outputAddr := to.Seed.Address(txConfig.ToAddressIndex).Address()

	unspentOutputs := AddressUnspentOutputs(t, from, inputAddr)
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
		// get the non-IOTA output
		mintOutputs := txn.Essence().Outputs().Filter(func(output ledgerstate.Output) bool {
			_, hasIOTA := output.Balances().Get(ledgerstate.ColorIOTA)
			return !hasIOTA
		})
		outputColor = blake2b.Sum256(mintOutputs[0].ID().Bytes())
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

func RequireBalancesEqual(t *testing.T, nodes []*framework.Node, addrBalance map[string]map[ledgerstate.Color]uint64) {
	for _, node := range nodes {
		for addrString, balances := range addrBalance {
			for color, balance := range balances {
				addr, err := ledgerstate.AddressFromBase58EncodedString(addrString)
				require.NoErrorf(t, err, "invalid address: %s", addrString)
				require.Equalf(t, balance, Balance(t, node, addr, color),
					"balance for color '%s' on address '%s' (node='%s') does not match", color, addr.Base58(), node)
			}
		}
	}
}

// CheckAddressOutputsFullyConsumed performs checks to make sure that on all given peers,
// the given addresses have no UTXOs.
func CheckAddressOutputsFullyConsumed(t *testing.T, peers []*framework.Node, addrs []string) {
	for _, peer := range peers {
		resp, err := peer.PostAddressUnspentOutputs(addrs)
		assert.NoError(t, err)
		for i, utxos := range resp.UnspentOutputs {
			assert.Len(t, utxos.Outputs, 0, "address %s should not have any UTXOs", addrs[i])
		}
	}
}

// ExpectedInclusionState is an expected inclusion state.
// All fields are optional.
type ExpectedInclusionState struct {
	// The optional confirmed state to check against.
	Confirmed *bool
	// The optional finalized state to check against.
	Finalized *bool
	// The optional conflict state to check against.
	Conflicting *bool
	// The optional solid state to check against.
	Solid *bool
	// The optional rejected state to check against.
	Rejected *bool
	// The optional liked state to check against.
	Liked *bool
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

// CheckTransactions performs checks to make sure that all peers have received all transactions.
// Optionally takes an expected inclusion state for all supplied transaction IDs and expected transaction
// data per transaction ID.
func CheckTransactions(t *testing.T, peers []*framework.Node, transactionIDs map[string]*ExpectedTransaction, checkSynchronized bool, expectedInclusionState ExpectedInclusionState) {
	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			require.Truef(t, info.TangleTime.Synced, "peer '%s' not synced", peer)
		}

		for txId, expectedTransaction := range transactionIDs {
			transaction, err := peer.GetTransaction(txId)
			require.NoError(t, err)

			inclusionState, err := peer.GetTransactionInclusionState(txId)
			require.NoError(t, err)

			metadata, err := peer.GetTransactionMetadata(txId)
			require.NoError(t, err)

			consensusData, err := peer.GetTransactionConsensusMetadata(txId)
			require.NoError(t, err)

			// check inclusion state
			if expectedInclusionState.Confirmed != nil {
				assert.Equal(t, *expectedInclusionState.Confirmed, inclusionState.Confirmed, "confirmed state doesn't match - tx %s - peer '%s'", txId, peer)
			}
			if expectedInclusionState.Conflicting != nil {
				assert.Equal(t, *expectedInclusionState.Conflicting, inclusionState.Conflicting, "conflict state doesn't match - tx %s - peer '%s'", txId, peer)
			}
			if expectedInclusionState.Solid != nil {
				assert.Equal(t, *expectedInclusionState.Solid, metadata.Solid, "solid state doesn't match - tx %s - peer '%s'", txId, peer)
			}
			if expectedInclusionState.Rejected != nil {
				assert.Equal(t, *expectedInclusionState.Rejected, inclusionState.Rejected, "rejected state doesn't match - tx %s - peer '%s'", txId, peer)
			}
			if expectedInclusionState.Liked != nil {
				assert.Equal(t, *expectedInclusionState.Liked, consensusData.Liked, "liked state doesn't match - tx %s - peer '%s'", txId, peer)
			}

			if expectedTransaction != nil {
				if expectedTransaction.Inputs != nil {
					assert.Equal(t, expectedTransaction.Inputs, transaction.Inputs, "inputs do not match - tx %s - peer '%s'", txId, peer)
				}
				if expectedTransaction.Outputs != nil {
					assert.Equal(t, expectedTransaction.Outputs, transaction.Outputs, "outputs do not match - tx %s - peer '%s'", txId, peer)
				}
				if expectedTransaction.UnlockBlocks != nil {
					assert.Equal(t, expectedTransaction.UnlockBlocks, transaction.UnlockBlocks, "signatures do not match - tx %s - peer '%s'", txId, peer)
				}
			}
		}
		// Transaction metadata
	}
}

func RequireTransactionsAvailable(t *testing.T, nodes []*framework.Node, transactionIDs []string, waitFor time.Duration, tick time.Duration) {
	missing := make(map[identity.ID]map[string]struct{}, len(nodes))
	for _, node := range nodes {
		missing[node.ID()] = make(map[string]struct{}, len(transactionIDs))
		for _, txID := range transactionIDs {
			missing[node.ID()][txID] = struct{}{}
		}
	}

	condition := func() bool {
		for _, node := range nodes {
			nodeMissing := missing[node.ID()]
			for txID := range nodeMissing {
				_, err := node.GetTransaction(txID)
				// retry, when the transaction could not be found
				if errors.Is(err, client.ErrNotFound) {
					continue
				}
				require.NoErrorf(t, err, "node=%s, txID=%, GetTransaction failed", node, txID)
				delete(nodeMissing, txID)
				if len(nodeMissing) == 0 {
					delete(missing, node.ID())
				}
			}
		}
		return len(missing) == 0
	}

	log.Printf("Waiting for %d transactions to become available...", len(transactionIDs))
	require.Eventuallyf(t, condition, waitFor, tick,
		"%d out of %d nodes did not receive all transactions", len(missing), len(nodes))
	log.Println("Waiting for transactions... done")
}

func RequireInclusionStateEqual(t *testing.T, nodes []*framework.Node, transactionIDs map[string]ExpectedInclusionState, waitFor time.Duration, tick time.Duration) {
	condition := func() bool {
		for _, node := range nodes {
			for txID, expInclState := range transactionIDs {
				inclusionState, err := node.GetTransactionInclusionState(txID)
				require.NoErrorf(t, err, "node=%s, txID=%, GetTransactionInclusionState failed")
				metadata, err := node.GetTransactionMetadata(txID)
				require.NoErrorf(t, err, "node=%s, txID=%, GetTransactionMetadata failed")
				consensusData, err := node.GetTransactionConsensusMetadata(txID)
				require.NoErrorf(t, err, "node=%s, txID=%, GetTransactionConsensusMetadata failed")

				// the inclusion state can change, so we should check all transactions every time
				if (expInclState.Confirmed != nil && *expInclState.Confirmed != inclusionState.Confirmed) ||
					(expInclState.Finalized != nil && *expInclState.Finalized != metadata.Finalized) ||
					(expInclState.Conflicting != nil && *expInclState.Conflicting != inclusionState.Conflicting) ||
					(expInclState.Solid != nil && *expInclState.Solid != metadata.Solid) ||
					(expInclState.Rejected != nil && *expInclState.Rejected != inclusionState.Rejected) ||
					(expInclState.Liked != nil && *expInclState.Liked != consensusData.Liked) {
					return false
				}
			}
		}
		return true
	}

	log.Printf("Waiting for %d transactions to reach the correct inclusion state...", len(transactionIDs))
	require.Eventually(t, condition, waitFor, tick)
	log.Println("Waiting for inclusion state... done")
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(ctx context.Context, t *testing.T, n Shutdowner) {
	log.Println("Shutting down network...")
	require.NoError(t, n.Shutdown(ctx))
	log.Println("Shutting down network... done")
}

func WaitForDeadline(t *testing.T) time.Duration {
	d, _ := t.Deadline()
	return time.Until(d.Add(-time.Minute))
}

func Context(ctx context.Context, t *testing.T) (context.Context, context.CancelFunc) {
	if d, ok := t.Deadline(); ok {
		return context.WithDeadline(ctx, d.Add(-time.Minute))
	}
	return context.WithCancel(ctx)
}

func Synced(t *testing.T, node *framework.Node) bool {
	info, err := node.Info()
	require.NoError(t, err)
	return info.TangleTime.Synced
}

func Balance(t *testing.T, peer *framework.Node, addr ledgerstate.Address, color ledgerstate.Color) uint64 {
	outputs := AddressUnspentOutputs(t, peer, addr)

	var sum uint64
	for _, unspent := range outputs {
		out, err := unspent.Output.ToLedgerstateOutput()
		require.NoError(t, err)
		balance, _ := out.Balances().Get(color)
		sum += balance
	}
	return sum
}

func AddressUnspentOutputs(t *testing.T, peer *framework.Node, addr ledgerstate.Address) []jsonmodels.WalletOutput {
	resp, err := peer.PostAddressUnspentOutputs([]string{addr.Base58()})
	require.NoErrorf(t, err, "node=%s, address=%s, PostAddressUnspentOutputs failed", peer, addr.Base58())
	require.Lenf(t, resp.UnspentOutputs, 1, "invalid response")
	require.Equalf(t, addr.Base58(), resp.UnspentOutputs[0].Address.Base58, "invalid response")

	return resp.UnspentOutputs[0].Outputs
}

func Mana(t *testing.T, node *framework.Node) jsonmodels.Mana {
	info, err := node.Info()
	require.NoError(t, err)
	return info.Mana
}

func SelectIndex(transaction *ledgerstate.Transaction, address ledgerstate.Address) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if address.Base58() == output.(*ledgerstate.SigLockedSingleOutput).Address().Base58() {
			return uint16(i)
		}
	}
	return
}
