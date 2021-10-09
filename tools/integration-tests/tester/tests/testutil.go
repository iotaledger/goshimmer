package tests

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
)

var faucetPoWDifficulty = framework.PeerConfig().Faucet.PowDifficulty

const (
	// Timeout denotes the default condition polling timout duration.
	Timeout = 3 * time.Minute
	// Tick denotes the default condition polling tick time.
	Tick = 500 * time.Millisecond

	shutdownGraceTime = time.Minute

	FaucetFundingOutputsAddrStart = 127 // the same as ledgerstate.MaxOutputCount

)

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
func AwaitInitialFaucetOutputsPrepared(t *testing.T, faucet *framework.Node) {
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
					if resp.UnspentOutputs[0].Outputs[0].InclusionState.Confirmed {
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
func AddressUnspentOutputs(t *testing.T, node *framework.Node, address ledgerstate.Address) []jsonmodels.WalletOutput {
	resp, err := node.PostAddressUnspentOutputs([]string{address.Base58()})
	require.NoErrorf(t, err, "node=%s, address=%s, PostAddressUnspentOutputs failed", node, address.Base58())
	require.Lenf(t, resp.UnspentOutputs, 1, "invalid response")
	require.Equalf(t, address.Base58(), resp.UnspentOutputs[0].Address.Base58, "invalid response")

	return resp.UnspentOutputs[0].Outputs
}

// Balance returns the total balance of color at address.
func Balance(t *testing.T, node *framework.Node, address ledgerstate.Address, color ledgerstate.Color) uint64 {
	unspentOutputs := AddressUnspentOutputs(t, node, address)

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
	return resp.ID, sent
}

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

// SendTransaction sends a transaction of value and color. It returns the transactionID and the error return by PostTransaction.
// If addrBalance is given the balance mutation are added to that map.
func SendTransaction(t *testing.T, from *framework.Node, to *framework.Node, color ledgerstate.Color, value uint64, txConfig TransactionConfig, addrBalance ...map[string]map[ledgerstate.Color]uint64) (string, error) {
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
				require.NoErrorf(t, err, "node=%s, messageID=%s, 'GetMessage' failed", node, messageID)
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
			unspent := AddressUnspentOutputs(t, node, addr)
			require.Empty(t, unspent, "address %s should not have any UTXOs", addr)
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

// RequireInclusionStateEqual asserts that all nodes have received the transaction and have correct expectedStates
// in waitFor time, periodically checking each tick.
func RequireInclusionStateEqual(t *testing.T, nodes []*framework.Node, expectedStates map[string]ExpectedInclusionState, waitFor time.Duration, tick time.Duration) {
	condition := func() bool {
		for _, node := range nodes {
			for txID, expInclState := range expectedStates {
				_, err := node.GetTransaction(txID)
				// retry, when the transaction could not be found
				if errors.Is(err, client.ErrNotFound) {
					continue
				}
				require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransaction' failed", node, txID)

				// the inclusion state can change, so we should check all transactions every time
				if !inclusionStateEqual(t, node, txID, expInclState) {
					return false
				}
			}
		}
		return true
	}

	log.Printf("Waiting for %d transactions to reach the correct inclusion state...", len(expectedStates))
	require.Eventually(t, condition, waitFor, tick)
	log.Println("Waiting for inclusion state... done")
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(ctx context.Context, t *testing.T, n interface {
	Shutdown(context.Context) error
}) {
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

func inclusionStateEqual(t *testing.T, node *framework.Node, txID string, expInclState ExpectedInclusionState) bool {
	inclusionState, err := node.GetTransactionInclusionState(txID)
	require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransactionInclusionState' failed")
	metadata, err := node.GetTransactionMetadata(txID)
	require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransactionMetadata' failed")
	consensusData, err := node.GetTransactionConsensusMetadata(txID)
	require.NoErrorf(t, err, "node=%s, txID=%, 'GetTransactionConsensusMetadata' failed")

	if (expInclState.Confirmed != nil && *expInclState.Confirmed != inclusionState.Confirmed) ||
		(expInclState.Finalized != nil && *expInclState.Finalized != metadata.Finalized) ||
		(expInclState.Conflicting != nil && *expInclState.Conflicting != inclusionState.Conflicting) ||
		(expInclState.Solid != nil && *expInclState.Solid != metadata.Solid) ||
		(expInclState.Rejected != nil && *expInclState.Rejected != inclusionState.Rejected) ||
		(expInclState.Liked != nil && *expInclState.Liked != consensusData.Liked) {
		return false
	}
	return true
}
