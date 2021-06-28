package tests

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
)

var (
	ErrMessageNotAvailableInTime     = errors.New("message was not available in time")
	ErrTransactionNotAvailableInTime = errors.New("transaction was not available in time")
	ErrTransactionStateNotSameInTime = errors.New("transaction state did not materialize in time")
	ErrNotSynced                     = errors.New("peers not synced")
)

const maxRetry = 50

// DataMessageSent defines a struct to identify from which issuer a data message was sent.
type DataMessageSent struct {
	number          int
	id              string
	data            []byte
	issuerPublicKey string
}

type Shutdowner interface {
	Shutdown() error
}

// TransactionConfig defines the configuration for a transaction.
type TransactionConfig struct {
	FromAddressIndex      uint64
	ToAddressIndex        uint64
	AccessManaPledgeID    identity.ID
	ConsensusManaPledgeID identity.ID
}

// SendDataMessagesOnRandomPeer sends data messages on a random peer and saves the sent message to a map.
func SendDataMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, numMessages int, idsMap ...map[string]DataMessageSent) map[string]DataMessageSent {
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
func SendDataMessage(t *testing.T, peer *framework.Peer, data []byte, number int) (string, DataMessageSent) {
	id, err := peer.Data(data)
	require.NoErrorf(t, err, "could not send message on %s", peer.String())

	sent := DataMessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewGenericDataPayload(data).Bytes(),
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return id, sent
}

// SendFaucetRequestOnRandomPeer sends a faucet request on a given peer and returns the id and a DataMessageSent struct.
func SendFaucetRequestOnRandomPeer(t *testing.T, peers []*framework.Peer, numMessages int) (ids map[string]DataMessageSent, addrBalance map[string]map[ledgerstate.Color]int64) {
	ids = make(map[string]DataMessageSent, numMessages)
	addrBalance = make(map[string]map[ledgerstate.Color]int64)

	for i := 0; i < numMessages; i++ {
		peer := peers[rand.Intn(len(peers))]
		addr := peer.Seed.Address(uint64(i)).Address()
		id, sent := SendFaucetRequest(t, peer, addr)
		ids[id] = sent
		addrBalance[addr.Base58()] = map[ledgerstate.Color]int64{
			ledgerstate.ColorIOTA: framework.ParaFaucetTokensPerRequest,
		}
	}

	return ids, addrBalance
}

// SendFaucetRequest sends a data message on a given peer and returns the id and a DataMessageSent struct. By default,
// it pledges mana to the peer making the request.
func SendFaucetRequest(t *testing.T, peer *framework.Peer, addr ledgerstate.Address, manaPledgeIDs ...string) (string, DataMessageSent) {
	peerID := base58.Encode(peer.ID().Bytes())
	aManaPledgeID, cManaPledgeID := peerID, peerID
	if len(manaPledgeIDs) > 1 {
		aManaPledgeID, cManaPledgeID = manaPledgeIDs[0], manaPledgeIDs[1]
	}

	resp, err := peer.SendFaucetRequest(addr.Base58(), framework.ParaPoWFaucetDifficulty, aManaPledgeID, cManaPledgeID)
	require.NoErrorf(t, err, "Could not send faucet request on %s", peer.String())

	sent := DataMessageSent{
		id:              resp.ID,
		data:            nil,
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return resp.ID, sent
}

// CheckForMessageIDs first waits for all messages to be available, and then performs checks to make sure that all peers received all given messages with
// their correct information.
func CheckForMessageIDs(t *testing.T, peers []*framework.Peer, messageIDs map[string]DataMessageSent, checkSynchronized bool, waitFor time.Duration) {
	missing, err := AwaitMessageAvailability(peers, messageIDs, waitFor)
	if err != nil {
		assert.NoError(t, err, "messages should have been available")
		for p, missingOnPeer := range missing {
			log.Printf("missing on peer %s:", p)
			for missingMessage := range missingOnPeer {
				log.Println("message id: ", missingMessage)
			}
		}
		return
	}

	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			assert.Truef(t, info.TangleTime.Synced, "Node %s is not synced", peer)
		}

		var idsSlice []string
		var respIDs []string
		for messageID := range messageIDs {
			idsSlice = append(idsSlice, messageID)

			resp, err := peer.GetMessage(messageID)
			require.NoError(t, err)
			respIDs = append(respIDs, resp.ID)

			respMetadata, err := peer.GetMessageMetadata(messageID)
			require.NoError(t, err)

			// check for general information
			msgSent := messageIDs[messageID]

			assert.Equalf(t, msgSent.issuerPublicKey, resp.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			if msgSent.data != nil {
				assert.Equalf(t, msgSent.data, resp.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			}

			assert.Truef(t, respMetadata.Solid, "messageID=%s, issuer=%s not solid in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
		}

		// check that all messages are present in response
		assert.ElementsMatchf(t, idsSlice, respIDs, "messages do not match sent in %s", peer.String())
	}
}

// AwaitMessageAvailability awaits until the given message IDs become available on all given peers or
// the max duration is reached. Returns a map of missing messages per peer. An error is returned if at least
// one peer does not have all specified messages available.
func AwaitMessageAvailability(peers []*framework.Peer, messageIDs map[string]DataMessageSent, waitFor time.Duration) (missing map[identity.ID]map[string]types.Empty, err error) {
	s := time.Now()
	var missingMu sync.RWMutex
	missing = map[identity.ID]map[string]types.Empty{}

	for i := 0; time.Since(s) < waitFor; time.Sleep(500 * time.Millisecond) {
		var wg sync.WaitGroup
		wg.Add(len(peers))

		for _, p := range peers {
			go func(p *framework.Peer) {
				defer wg.Done()

				missingMu.RLock()
				m, has := missing[p.ID()]
				missingMu.RUnlock()

				// do not request messages again for this peer if a previous iteration did not yield any missing messages
				if i > 0 && !has {
					return
				}

				for messageID := range messageIDs {
					_, err := p.GetMessage(messageID)
					if err == nil {
						if has {
							delete(m, messageID)
							if len(m) == 0 {
								missingMu.Lock()
								delete(missing, p.ID())
								missingMu.Unlock()
							}
						}
						continue
					}

					if !has {
						m = map[string]types.Empty{}
					}
					m[messageID] = types.Void

					missingMu.Lock()
					missing[p.ID()] = m
					missingMu.Unlock()
				}
			}(p)
		}
		wg.Wait()

		if len(missing) == 0 {
			return missing, nil
		}
		i++
	}
	return missing, ErrMessageNotAvailableInTime
}

// SendTransactionFromFaucet sends funds to peers from the faucet, sends back the remainder to faucet, and returns the transaction ID.
func SendTransactionFromFaucet(t *testing.T, peers []*framework.Peer, sentValue int64) (txIds []string, addrBalance map[string]map[ledgerstate.Color]int64) {
	// initiate addrBalance map
	addrBalance = make(map[string]map[ledgerstate.Color]int64)
	for _, p := range peers {
		addr := p.Seed.Address(0).Address().Base58()
		addrBalance[addr] = make(map[ledgerstate.Color]int64)
	}

	faucetPeer := peers[0]

	// faucet keeps remaining amount on address 0
	addrBalance[faucetPeer.Seed.Address(0).Address().Base58()][ledgerstate.ColorIOTA] = int64(framework.GenesisTokenAmount - framework.ParaFaucetPreparedOutputsCount*int(framework.ParaFaucetTokensPerRequest))
	var i uint64
	// faucet has split genesis output into n bits of 1337 each and remainder on 0
	for i = 1; i < uint64(len(peers)); i++ {
		faucetAddrStr := faucetPeer.Seed.Address(i).Address().Base58()
		addrBalance[faucetAddrStr] = make(map[ledgerstate.Color]int64)
		// get faucet balances
		unspentOutputs, err := faucetPeer.PostAddressUnspentOutputs([]string{faucetAddrStr})
		require.NoErrorf(t, err, "could not get unspent outputs on %s", faucetPeer.String())
		out, err := unspentOutputs.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
		require.NoError(t, err)
		balanceValue, exist := out.Balances().Get(ledgerstate.ColorIOTA)
		assert.Equal(t, true, exist)
		addrBalance[faucetAddrStr][ledgerstate.ColorIOTA] = int64(balanceValue)

		// send funds to other peers
		fail, txId := SendIotaTransaction(t, faucetPeer, peers[i], addrBalance, sentValue, TransactionConfig{
			FromAddressIndex:      i,
			ToAddressIndex:        0,
			AccessManaPledgeID:    peers[i].ID(),
			ConsensusManaPledgeID: peers[i].ID(),
		})
		require.False(t, fail)
		txIds = append(txIds, txId)

		// let the transaction propagate
		time.Sleep(3 * time.Second)
	}

	return
}

// SendTransactionOnRandomPeer sends sentValue amount of IOTA tokens from/to a random peer, mutates the given balance map and returns the transaction IDs.
func SendTransactionOnRandomPeer(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[ledgerstate.Color]int64, numMessages int, sentValue int64) (txIds []string) {
	counter := 0
	for i := 0; i < numMessages; i++ {
		from := rand.Intn(len(peers))
		to := rand.Intn(len(peers))
		fail, txId := SendIotaTransaction(t, peers[from], peers[to], addrBalance, sentValue, TransactionConfig{})
		if fail {
			i--
			counter++
			if counter >= maxRetry {
				return
			}
			continue
		}

		// attach tx id
		txIds = append(txIds, txId)

		// let the transaction propagate
		time.Sleep(3 * time.Second)
	}

	return
}

// SendIotaTransaction sends sentValue amount of IOTA tokens and remainders from and to a given peer and returns the fail flag and the transaction ID.
// Every peer sends and receives the transaction on the address of index 0.
// Optionally, the nodes to pledge access and consensus mana can be specified.
func SendIotaTransaction(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]map[ledgerstate.Color]int64, sentValue int64, txConfig TransactionConfig) (fail bool, txId string) {
	inputAddr := from.Seed.Address(txConfig.FromAddressIndex).Address()
	outputAddr := to.Seed.Address(txConfig.ToAddressIndex).Address()

	// prepare inputs
	resp, err := from.PostAddressUnspentOutputs([]string{inputAddr.Base58()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", from.String())

	// abort if no unspent outputs
	if len(resp.UnspentOutputs[0].Outputs) == 0 {
		return true, ""
	}
	out, err := resp.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	balanceValue, exist := out.Balances().Get(ledgerstate.ColorIOTA)
	assert.Equal(t, true, exist)
	availableValue := int64(balanceValue)

	// abort if the balance is not enough
	if availableValue < sentValue {
		return true, ""
	}

	out, err = resp.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoErrorf(t, err, "invalid unspent outputs ID on %s", from.String())
	input := ledgerstate.NewUTXOInput(out.ID())
	if inputAddr == outputAddr {
		sentValue = availableValue
	}

	// set balances
	var outputs ledgerstate.Outputs
	output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: uint64(sentValue),
	}), outputAddr)
	outputs = append(outputs, output)

	// handle remainder address
	if availableValue > sentValue {
		output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
			ledgerstate.ColorIOTA: uint64(availableValue - sentValue),
		}), inputAddr)
		outputs = append(outputs, output)
	}
	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), txConfig.AccessManaPledgeID, txConfig.ConsensusManaPledgeID, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(outputs...))
	sig := ledgerstate.NewED25519Signature(from.KeyPair(txConfig.FromAddressIndex).PublicKey, from.KeyPair(txConfig.FromAddressIndex).PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	txn := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})

	// send transaction
	respTx, err := from.PostTransaction(txn.Bytes())
	if err != nil {
		fmt.Println(fmt.Errorf("could not send transaction on %s: %w", from.String(), err).Error())
		return true, ""
	}
	txId = respTx.TransactionID
	addrBalance[inputAddr.Base58()][ledgerstate.ColorIOTA] -= sentValue
	addrBalance[outputAddr.Base58()][ledgerstate.ColorIOTA] += sentValue
	return false, txId
}

// SendColoredTransaction sends IOTA and colored tokens from and to a given peer and returns the ok flag and transaction ID.
// 1. Get the first unspent outputs of `from`
// 2. Send 50 IOTA and 50 ColorMint to `to`
func SendColoredTransaction(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]map[ledgerstate.Color]int64, txConfig TransactionConfig) (fail bool, txId string) {
	var sentIOTAValue int64 = 50
	var sentMintValue int64 = 50
	var balanceList []coloredBalance
	inputAddr := from.Seed.Address(txConfig.FromAddressIndex).Address()
	outputAddr := to.Seed.Address(txConfig.ToAddressIndex).Address()

	// prepare inputs
	resp, err := from.PostAddressUnspentOutputs([]string{inputAddr.Base58()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", from.String())

	// abort if no unspent outputs
	if len(resp.UnspentOutputs[0].Outputs) == 0 {
		return false, ""
	}

	out, err := resp.UnspentOutputs[0].Outputs[0].Output.ToLedgerstateOutput()
	require.NoError(t, err)
	input := ledgerstate.NewUTXOInput(out.ID())

	// prepare output
	output := ledgerstate.NewSigLockedColoredOutput(ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
		ledgerstate.ColorIOTA: uint64(sentIOTAValue),
		ledgerstate.ColorMint: uint64(sentMintValue),
	}), outputAddr)

	txEssence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, ledgerstate.NewInputs(input), ledgerstate.NewOutputs(output))

	// sign transaction
	sig := ledgerstate.NewED25519Signature(from.KeyPair(txConfig.FromAddressIndex).PublicKey, from.KeyPair(txConfig.FromAddressIndex).PrivateKey.Sign(txEssence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(sig)
	txn := ledgerstate.NewTransaction(txEssence, ledgerstate.UnlockBlocks{unlockBlock})

	// set expected balances for test, credit from inputAddr
	balanceList = append(balanceList, coloredBalance{
		Color:   ledgerstate.ColorIOTA,
		Balance: -(sentIOTAValue + sentMintValue),
	})
	// set expected balances for test, debit to outputAddr
	balanceList = append(balanceList, coloredBalance{
		Color:   ledgerstate.ColorIOTA,
		Balance: sentIOTAValue,
	})
	balanceList = append(balanceList, coloredBalance{
		Color:   ledgerstate.ColorMint,
		Balance: sentMintValue,
	})

	// send transaction
	respTx, err := from.PostTransaction(txn.Bytes())
	if err != nil {
		fmt.Println(fmt.Errorf("could not send transaction on %s: %w", from.String(), err).Error())
		return true, ""
	}
	txId = respTx.TransactionID

	// update balance list
	updateBalanceList(addrBalance, balanceList, inputAddr.Base58(), outputAddr.Base58(), txn.Essence().Outputs()[0])

	return false, txId
}

// updateBalanceList updates the token amount map with given peers and balances.
// If the value of balance is negative, it is the balance to be deducted from peer from, else it is deposited to peer to.
// If the color is ledgerstate.ColorMint, it is recolored / tokens are minted.
func updateBalanceList(addrBalance map[string]map[ledgerstate.Color]int64, balances []coloredBalance, from, to string, output ledgerstate.Output) {
	for _, b := range balances {
		color := b.Color
		value := b.Balance
		if value < 0 {
			// deduct
			addrBalance[from][color] += value
			continue
		}
		// deposit
		if color == ledgerstate.ColorMint {
			color = blake2b.Sum256(output.ID().Bytes())
			addrBalance[to][color] = value
			continue
		}
		addrBalance[to][color] += value
	}
	return
}

func getColorFromString(colorStr string) (color ledgerstate.Color) {
	if colorStr == "IOTA" {
		color = ledgerstate.ColorIOTA
	} else {
		t, _ := ledgerstate.TransactionIDFromBase58(colorStr)
		color, _, _ = ledgerstate.ColorFromBytes(t.Bytes())
	}
	return
}

// CheckBalances performs checks to make sure that all peers have the same ledger state.
func CheckBalances(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[ledgerstate.Color]int64) {
	for _, peer := range peers {
		for addr, b := range addrBalance {
			sum := make(map[ledgerstate.Color]int64)
			resp, err := peer.PostAddressUnspentOutputs([]string{addr})
			require.NoError(t, err)
			assert.Equal(t, addr, resp.UnspentOutputs[0].Address.Base58)

			// calculate the balances of each colored coin
			for _, unspents := range resp.UnspentOutputs[0].Outputs {
				out, err2 := unspents.Output.ToLedgerstateOutput()
				require.NoError(t, err2)
				out.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
					sum[color] += int64(balance)
					return true
				})
			}
			// check balances
			for color, value := range sum {
				assert.Equalf(t, b[color], value, "balance for color '%s' on address '%s' (peer='%s') does not match", color, addr, peer)
			}
		}
	}
}

// CheckAddressOutputsFullyConsumed performs checks to make sure that on all given peers,
// the given addresses have no UTXOs.
func CheckAddressOutputsFullyConsumed(t *testing.T, peers []*framework.Peer, addrs []string) {
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
	SolidityType *string
	// The optional rejected state to check against.
	Rejected *bool
	// The optional liked state to check against.
	Liked *bool
	// The optional preferred state to check against.
	Preferred *bool
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

// Solid returns a pointer to the string representation of the Solid SolidityType.
func Solid() *string {
	x := ledgerstate.Solid.String()
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
func CheckTransactions(t *testing.T, peers []*framework.Peer, transactionIDs map[string]*ExpectedTransaction, checkSynchronized bool, expectedInclusionState ExpectedInclusionState) {
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
			if expectedInclusionState.SolidityType != nil {
				assert.Equal(t, *expectedInclusionState.SolidityType, metadata.SolidityType, "solid state doesn't match - tx %s - peer '%s'", txId, peer)
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

// AwaitTransactionAvailability awaits until the given transaction IDs become available on all given peers or
// the max duration is reached. Returns a map of missing transactions per peer. An error is returned if at least
// one peer does not have all specified transactions available.
func AwaitTransactionAvailability(peers []*framework.Peer, transactionIDs []string, maxAwait time.Duration) (missing map[string]map[string]types.Empty, err error) {
	s := time.Now()
	var missingMu sync.Mutex
	missing = map[string]map[string]types.Empty{}
	for ; time.Since(s) < maxAwait; time.Sleep(500 * time.Millisecond) {
		var wg sync.WaitGroup
		wg.Add(len(peers))
		counter := int32(len(peers) * len(transactionIDs))
		for _, p := range peers {
			go func(p *framework.Peer) {
				defer wg.Done()
				for _, txID := range transactionIDs {
					_, err := p.GetTransaction(txID)
					if err == nil {
						missingMu.Lock()
						m, has := missing[p.ID().String()]
						if has {
							delete(m, txID)
							if len(m) == 0 {
								delete(missing, p.ID().String())
							}
						}
						missingMu.Unlock()
						atomic.AddInt32(&counter, -1)
						continue
					}
					missingMu.Lock()
					m, has := missing[p.ID().String()]
					if !has {
						m = map[string]types.Empty{}
					}
					m[txID] = types.Empty{}
					missing[p.ID().String()] = m
					missingMu.Unlock()
				}
			}(p)
		}
		wg.Wait()
		if counter == 0 {
			// everything available
			return missing, nil
		}
	}
	return missing, ErrTransactionNotAvailableInTime
}

// AwaitTransactionInclusionState awaits on all given peers until the specified transactions
// have the expected state or max duration is reached. This function does not gracefully
// handle the transactions not existing on the given peers, therefore it must be ensured
// the the transactions exist beforehand.
func AwaitTransactionInclusionState(peers []*framework.Peer, transactionIDs map[string]ExpectedInclusionState, maxAwait time.Duration) error {
	s := time.Now()
	for ; time.Since(s) < maxAwait; time.Sleep(1 * time.Second) {
		var wg sync.WaitGroup
		wg.Add(len(peers))
		counter := int32(len(peers) * len(transactionIDs))
		for _, p := range peers {
			go func(p *framework.Peer) {
				defer wg.Done()
				for txID := range transactionIDs {
					inclusionState, err := p.GetTransactionInclusionState(txID)
					if err != nil {
						continue
					}
					metadata, err := p.GetTransactionMetadata(txID)
					if err != nil {
						continue
					}
					consensusData, err := p.GetTransactionConsensusMetadata(txID)
					if err != nil {
						continue
					}
					expInclState := transactionIDs[txID]
					if expInclState.Confirmed != nil && *expInclState.Confirmed != inclusionState.Confirmed {
						continue
					}
					if expInclState.Conflicting != nil && *expInclState.Conflicting != inclusionState.Conflicting {
						continue
					}
					if expInclState.Finalized != nil && *expInclState.Finalized != metadata.Finalized {
						continue
					}
					if expInclState.Liked != nil && *expInclState.Liked != consensusData.Liked {
						continue
					}
					if expInclState.Rejected != nil && *expInclState.Rejected != inclusionState.Rejected {
						continue
					}
					if expInclState.SolidityType != nil && *expInclState.SolidityType != metadata.SolidityType {
						continue
					}
					atomic.AddInt32(&counter, -1)
				}
			}(p)
		}
		wg.Wait()
		if counter == 0 {
			// everything available
			return nil
		}
	}
	return ErrTransactionStateNotSameInTime
}

// AwaitSync waits until the given peers are in synced.
func AwaitSync(t *testing.T, peers []*framework.Peer, maxAwait time.Duration) error {
	s := time.Now()
	for ; time.Since(s) < maxAwait; time.Sleep(1 * time.Second) {
		// check that the peer sees itself as synchronized
		allSynced := true
		for _, peer := range peers {
			info, err := peer.Info()
			require.NoError(t, err)
			allSynced = allSynced && info.TangleTime.Synced
		}
		if allSynced {
			return nil
		}
	}
	return ErrNotSynced
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n Shutdowner) {
	err := n.Shutdown()
	require.NoError(t, err)
}

type coloredBalance struct {
	Color   ledgerstate.Color
	Balance int64
}

func (b coloredBalance) String() string {
	return stringify.Struct("coloredBalance",
		stringify.StructField("Color", b.Color),
		stringify.StructField("Balance", b.Balance),
	)
}

func SelectIndex(transaction *ledgerstate.Transaction, address ledgerstate.Address) (index uint16) {
	for i, output := range transaction.Essence().Outputs() {
		if address.Base58() == output.(*ledgerstate.SigLockedSingleOutput).Address().Base58() {
			return uint16(i)
		}
	}
	return
}

func GetSnapshot() *ledgerstate.Snapshot {
	snapshot := &ledgerstate.Snapshot{}
	f, err := os.Open("/tmp/assets/7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin")
	if err != nil {
		panic(fmt.Sprintln("can not open snapshot file: ", err))
	}
	if _, err := snapshot.ReadFrom(f); err != nil {
		panic(fmt.Sprintln("could not read snapshot file: ", err))
	}
	return snapshot
}
