package tests

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/plugins/webapi/value/utils"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	ErrTransactionNotAvailableInTime = errors.New("transaction was not available in time")
	ErrTransactionStateNotSameInTime = errors.New("transaction state did not materialize in time")
)

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

// SendDataMessagesOnRandomPeer sends data messages on a random peer and saves the sent message to a map.
func SendDataMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, numMessages int, idsMap ...map[string]DataMessageSent) map[string]DataMessageSent {
	var ids map[string]DataMessageSent
	if len(idsMap) > 0 {
		ids = idsMap[0]
	} else {
		ids = make(map[string]DataMessageSent, numMessages)
	}

	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("Test%d", i))

		peer := peers[rand.Intn(len(peers))]
		id, sent := SendDataMessage(t, peer, data, i)

		ids[id] = sent
	}

	return ids
}

// SendDataMessage sends a data message on a given peer and returns the id and a DataMessageSent struct.
func SendDataMessage(t *testing.T, peer *framework.Peer, data []byte, number int) (string, DataMessageSent) {
	id, err := peer.Data(data)
	require.NoErrorf(t, err, "Could not send message on %s", peer.String())

	sent := DataMessageSent{
		number: number,
		id:     id,
		// save payload to be able to compare API response
		data:            payload.NewData(data).Bytes(),
		issuerPublicKey: peer.Identity.PublicKey().String(),
	}
	return id, sent
}

// CheckForMessageIds performs checks to make sure that all peers received all given messages defined in ids.
func CheckForMessageIds(t *testing.T, peers []*framework.Peer, ids map[string]DataMessageSent, checkSynchronized bool) {
	var idsSlice []string
	for id := range ids {
		idsSlice = append(idsSlice, id)
	}

	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			require.True(t, info.Synced)
		}

		resp, err := peer.FindMessageByID(idsSlice)
		require.NoError(t, err)

		// check that all messages are present in response
		respIDs := make([]string, len(resp.Messages))
		for i, msg := range resp.Messages {
			respIDs[i] = msg.ID
		}
		assert.ElementsMatchf(t, idsSlice, respIDs, "messages do not match sent in %s", peer.String())

		// check for general information
		for _, msg := range resp.Messages {
			msgSent := ids[msg.ID]

			assert.Equalf(t, msgSent.issuerPublicKey, msg.IssuerPublicKey, "messageID=%s, issuer=%s not correct issuer in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Equalf(t, msgSent.data, msg.Payload, "messageID=%s, issuer=%s data not equal in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
			assert.Truef(t, msg.Metadata.Solid, "messageID=%s, issuer=%s not solid in %s.", msgSent.id, msgSent.issuerPublicKey, peer.String())
		}
	}
}

// SendValueMessagesOnFaucet sends funds to peers from the faucet and returns the transaction ID.
func SendValueMessagesOnFaucet(t *testing.T, peers []*framework.Peer) (txIds []string, addrBalance map[string]map[balance.Color]int64) {
	// initiate addrBalance map
	addrBalance = make(map[string]map[balance.Color]int64)
	for _, p := range peers {
		addr := p.Seed().Address(0).String()
		addrBalance[addr] = make(map[balance.Color]int64)
		addrBalance[addr][balance.ColorIOTA] = 0
	}

	faucetPeer := peers[0]
	faucetAddrStr := faucetPeer.Seed().Address(0).String()

	// get faucet balances
	unspentOutputs, err := faucetPeer.GetUnspentOutputs([]string{faucetAddrStr})
	require.NoErrorf(t, err, "Could not get unspent outputs on %s", faucetPeer.String())
	addrBalance[faucetAddrStr][balance.ColorIOTA] = unspentOutputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	// send funds to other peers
	for i := 1; i < len(peers); i++ {
		fail, txId := SendIotaValueMessages(t, faucetPeer, peers[i], addrBalance)
		require.False(t, fail)
		txIds = append(txIds, txId)

		// let the transaction propagate
		time.Sleep(1 * time.Second)
	}
	return
}

// SendValueMessagesOnRandomPeer sends IOTA tokens on random peer and saves the sent message token to a map.
func SendValueMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[balance.Color]int64, numMessages int) (txIds []string) {
	for i := 0; i < numMessages; i++ {
		from := rand.Intn(len(peers))
		to := rand.Intn(len(peers))
		fail, txId := SendIotaValueMessages(t, peers[from], peers[to], addrBalance)
		if fail {
			i--
			continue
		}

		// attach tx id
		txIds = append(txIds, txId)

		// let the transaction propagate
		time.Sleep(1 * time.Second)
	}

	return
}

// SendIotaValueMessages sends IOTA token from and to a given peer and returns the transaction ID.
// The same addresses are used in each round
func SendIotaValueMessages(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]map[balance.Color]int64) (fail bool, txId string) {
	var sentValue int64 = 100
	sigScheme := signaturescheme.ED25519(*from.Seed().KeyPair(0))
	inputAddr := from.Seed().Address(0)
	outputAddr := to.Seed().Address(0)

	// prepare inputs
	resp, err := from.GetUnspentOutputs([]string{inputAddr.String()})
	require.NoErrorf(t, err, "Could not get unspent outputs on %s", from.String())

	// abort if no unspent outputs
	if len(resp.UnspentOutputs[0].OutputIDs) == 0 {
		return true, ""
	}
	availableValue := resp.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	//abort if the balance is not enough
	if availableValue < sentValue {
		return true, ""
	}

	out, err := transaction.OutputIDFromBase58(resp.UnspentOutputs[0].OutputIDs[0].ID)
	require.NoErrorf(t, err, "Invalid unspent outputs ID on %s", from.String())
	inputs := transaction.NewInputs([]transaction.OutputID{out}...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}
	if inputAddr == outputAddr {
		sentValue = availableValue
	}

	// set balances
	outmap[outputAddr] = []*balance.Balance{balance.New(balance.ColorIOTA, sentValue)}
	outputs := transaction.NewOutputs(outmap)

	// handle remain address
	if availableValue > sentValue {
		outputs.Add(inputAddr, []*balance.Balance{balance.New(balance.ColorIOTA, availableValue-sentValue)})
	}

	// sign transaction
	txn := transaction.New(inputs, outputs).Sign(sigScheme)

	// send transaction
	txId, err = from.SendTransaction(txn.Bytes())
	require.NoErrorf(t, err, "Could not send transaction on %s", from.String())

	addrBalance[inputAddr.String()][balance.ColorIOTA] -= sentValue
	addrBalance[outputAddr.String()][balance.ColorIOTA] += sentValue

	return false, txId
}

// SendColoredValueMessagesOnRandomPeer sends colored token on a random peer and saves the sent token to a map.
func SendColoredValueMessagesOnRandomPeer(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[balance.Color]int64, numMessages int) (txIds []string) {
	for i := 0; i < numMessages; i++ {
		from := rand.Intn(len(peers))
		to := rand.Intn(len(peers))
		fail, txId := SendColoredValueMessage(t, peers[from], peers[to], addrBalance)
		if fail {
			i--
			continue
		}

		// attach tx id
		txIds = append(txIds, txId)

		// let the transaction propagate
		time.Sleep(1 * time.Second)
	}

	return
}

// SendColoredValueMessage sends a colored tokens from and to a given peer and returns the transaction ID.
// The same addresses are used in each round
func SendColoredValueMessage(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]map[balance.Color]int64) (fail bool, txId string) {
	sigScheme := signaturescheme.ED25519(*from.Seed().KeyPair(0))
	inputAddr := from.Seed().Address(0)
	outputAddr := to.Seed().Address(0)

	// prepare inputs
	resp, err := from.GetUnspentOutputs([]string{inputAddr.String()})
	require.NoErrorf(t, err, "Could not get unspent outputs on %s", from.String())

	// abort if no unspent outputs
	if len(resp.UnspentOutputs[0].OutputIDs) == 0 {
		return true, ""
	}

	out, err := transaction.OutputIDFromBase58(resp.UnspentOutputs[0].OutputIDs[0].ID)
	require.NoErrorf(t, err, "Invalid unspent outputs ID on %s", from.String())
	inputs := transaction.NewInputs([]transaction.OutputID{out}...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}
	bs := []*balance.Balance{}
	var outputs *transaction.Outputs
	var availableIOTA int64
	availableBalances := resp.UnspentOutputs[0].OutputIDs[0].Balances
	newColor := false

	// set balances
	if len(availableBalances) > 1 {
		// the balances contain more than one color, send it all
		for _, b := range availableBalances {
			value := b.Value
			color := getColorFromString(b.Color)
			bs = append(bs, balance.New(color, value))

			// update balance list
			addrBalance[inputAddr.String()][color] -= value
			if _, ok := addrBalance[outputAddr.String()][color]; ok {
				addrBalance[outputAddr.String()][color] += value
			} else {
				addrBalance[outputAddr.String()][color] = value
			}
		}
	} else {
		// create new colored token if inputs only contain IOTA
		// half of availableIota tokens remain IOTA, else get recolored
		newColor = true
		availableIOTA = availableBalances[0].Value

		bs = append(bs, balance.New(balance.ColorIOTA, availableIOTA/2))
		bs = append(bs, balance.New(balance.ColorNew, availableIOTA/2))

		// update balance list
		addrBalance[inputAddr.String()][balance.ColorIOTA] -= availableIOTA
		addrBalance[outputAddr.String()][balance.ColorIOTA] += availableIOTA / 2
	}
	outmap[outputAddr] = bs

	outputs = transaction.NewOutputs(outmap)

	// sign transaction
	txn := transaction.New(inputs, outputs).Sign(sigScheme)

	// send transaction
	txId, err = from.SendTransaction(txn.Bytes())
	require.NoErrorf(t, err, "Could not send transaction on %s", from.String())

	// FIXME: the new color should be txn ID
	if newColor {
		if _, ok := addrBalance[outputAddr.String()][balance.ColorNew]; ok {
			addrBalance[outputAddr.String()][balance.ColorNew] += availableIOTA / 2
		} else {
			addrBalance[outputAddr.String()][balance.ColorNew] = availableIOTA / 2
		}
		//addrBalance[outputAddr.String()][getColorFromString(txId)] = availableIOTA / 2
	}
	return false, txId
}

func getColorFromString(colorStr string) (color balance.Color) {
	if colorStr == "IOTA" {
		color = balance.ColorIOTA
	} else {
		t, _ := transaction.IDFromBase58(colorStr)
		color, _, _ = balance.ColorFromBytes(t.Bytes())
	}
	return
}

// CheckBalances performs checks to make sure that all peers have the same ledger state.
func CheckBalances(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[balance.Color]int64) {
	for _, peer := range peers {
		for addr, b := range addrBalance {
			sum := make(map[balance.Color]int64)
			resp, err := peer.GetUnspentOutputs([]string{addr})
			require.NoError(t, err)
			assert.Equal(t, addr, resp.UnspentOutputs[0].Address)

			// calculate the balances of each color coin
			for _, unspents := range resp.UnspentOutputs[0].OutputIDs {
				for _, b := range unspents.Balances {
					color := getColorFromString(b.Color)
					if _, ok := sum[color]; ok {
						sum[color] += b.Value
					} else {
						sum[color] = b.Value
					}
				}
			}

			// check balances
			for color, value := range sum {
				assert.Equal(t, b[color], value)
			}
		}
	}
}

// CheckAddressOutputsFullyConsumed performs checks to make sure that on all given peers,
// the given addresses have no UTXOs.
func CheckAddressOutputsFullyConsumed(t *testing.T, peers []*framework.Peer, addrs []string) {
	for _, peer := range peers {
		resp, err := peer.GetUnspentOutputs(addrs)
		assert.NoError(t, err)
		assert.Len(t, resp.Error, 0)
		for i, utxos := range resp.UnspentOutputs {
			assert.Len(t, utxos.OutputIDs, 0, "address %s should not have any UTXOs", addrs[i])
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

// ExpectedTransaction defines the expected data of a transaction.
// All fields are optional.
type ExpectedTransaction struct {
	// The optional input IDs to check against.
	Inputs *[]string
	// The optional outputs to check against.
	Outputs *[]utils.Output
	// The optional signature to check against.
	Signature *[]byte
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
			require.True(t, info.Synced)
		}

		for txId, expectedTransaction := range transactionIDs {
			resp, err := peer.GetTransactionByID(txId)
			require.NoError(t, err)

			// check inclusion state
			if expectedInclusionState.Confirmed != nil {
				assert.Equal(t, *expectedInclusionState.Confirmed, resp.InclusionState.Confirmed, "confirmed state doesn't match - %s", txId)
			}
			if expectedInclusionState.Conflicting != nil {
				assert.Equal(t, *expectedInclusionState.Conflicting, resp.InclusionState.Conflicting, "conflict state doesn't match - %s", txId)
			}
			if expectedInclusionState.Solid != nil {
				assert.Equal(t, *expectedInclusionState.Solid, resp.InclusionState.Solid, "solid state doesn't match - %s", txId)
			}
			if expectedInclusionState.Rejected != nil {
				assert.Equal(t, *expectedInclusionState.Rejected, resp.InclusionState.Rejected, "rejected state doesn't match - %s", txId)
			}
			if expectedInclusionState.Liked != nil {
				assert.Equal(t, *expectedInclusionState.Liked, resp.InclusionState.Liked, "liked state doesn't match - %s", txId)
			}
			if expectedInclusionState.Preferred != nil {
				assert.Equal(t, *expectedInclusionState.Preferred, resp.InclusionState.Preferred, "preferred state doesn't match - %s", txId)
			}

			if expectedTransaction != nil {
				if expectedTransaction.Inputs != nil {
					assert.Equal(t, *expectedTransaction.Inputs, resp.Transaction.Inputs, "inputs do not match - %s", txId)
				}
				if expectedTransaction.Outputs != nil {
					assert.Equal(t, *expectedTransaction.Outputs, resp.Transaction.Outputs, "outputs do not match - %s", txId)
				}
				if expectedTransaction.Signature != nil {
					assert.Equal(t, *expectedTransaction.Signature, resp.Transaction.Signature, "signatures do not match - %s", txId)
				}
			}
		}
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
					_, err := p.GetTransactionByID(txID)
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
					tx, err := p.GetTransactionByID(txID)
					if err != nil {
						continue
					}
					expInclState := transactionIDs[txID]
					if expInclState.Confirmed != nil && *expInclState.Confirmed != tx.InclusionState.Confirmed {
						continue
					}
					if expInclState.Conflicting != nil && *expInclState.Conflicting != tx.InclusionState.Conflicting {
						continue
					}
					if expInclState.Finalized != nil && *expInclState.Finalized != tx.InclusionState.Finalized {
						continue
					}
					if expInclState.Liked != nil && *expInclState.Liked != tx.InclusionState.Liked {
						continue
					}
					if expInclState.Preferred != nil && *expInclState.Preferred != tx.InclusionState.Preferred {
						continue
					}
					if expInclState.Rejected != nil && *expInclState.Rejected != tx.InclusionState.Rejected {
						continue
					}
					if expInclState.Solid != nil && *expInclState.Solid != tx.InclusionState.Solid {
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

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n Shutdowner) {
	err := n.Shutdown()
	require.NoError(t, err)
}
