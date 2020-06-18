package tests

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoErrorf(t, err, "could not send message on %s", peer.String())

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

// SendTransactionFromFaucet sends funds to peers from the faucet, sends back the remainder to faucet, and returns the transaction ID.
func SendTransactionFromFaucet(t *testing.T, peers []*framework.Peer, sentValue int64) (txIds []string, addrBalance map[string]map[balance.Color]int64) {
	// initiate addrBalance map
	addrBalance = make(map[string]map[balance.Color]int64)
	for _, p := range peers {
		addr := p.Seed().Address(0).String()
		addrBalance[addr] = make(map[balance.Color]int64)
	}

	faucetPeer := peers[0]
	faucetAddrStr := faucetPeer.Seed().Address(0).String()

	// get faucet balances
	unspentOutputs, err := faucetPeer.GetUnspentOutputs([]string{faucetAddrStr})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", faucetPeer.String())
	addrBalance[faucetAddrStr][balance.ColorIOTA] = unspentOutputs.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	// send funds to other peers
	for i := 1; i < len(peers); i++ {
		fail, txId := SendIotaTransaction(t, faucetPeer, peers[i], addrBalance, sentValue)
		require.False(t, fail)
		txIds = append(txIds, txId)

		// let the transaction propagate
		time.Sleep(3 * time.Second)
	}

	return
}

// SendTransactionOnRandomPeer sends sentValue amount of IOTA tokens from/to a random peer, mutates the given balance map and returns the transaction IDs.
func SendTransactionOnRandomPeer(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[balance.Color]int64, numMessages int, sentValue int64) (txIds []string) {
	counter := 0
	for i := 0; i < numMessages; i++ {
		from := rand.Intn(len(peers))
		to := rand.Intn(len(peers))
		fail, txId := SendIotaTransaction(t, peers[from], peers[to], addrBalance, sentValue)
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
func SendIotaTransaction(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]map[balance.Color]int64, sentValue int64) (fail bool, txId string) {
	sigScheme := signaturescheme.ED25519(*from.Seed().KeyPair(0))
	inputAddr := from.Seed().Address(0)
	outputAddr := to.Seed().Address(0)

	// prepare inputs
	resp, err := from.GetUnspentOutputs([]string{inputAddr.String()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", from.String())

	// abort if no unspent outputs
	if len(resp.UnspentOutputs[0].OutputIDs) == 0 {
		return true, ""
	}
	availableValue := resp.UnspentOutputs[0].OutputIDs[0].Balances[0].Value

	// abort if the balance is not enough
	if availableValue < sentValue {
		return true, ""
	}

	out, err := transaction.OutputIDFromBase58(resp.UnspentOutputs[0].OutputIDs[0].ID)
	require.NoErrorf(t, err, "invalid unspent outputs ID on %s", from.String())
	inputs := transaction.NewInputs([]transaction.OutputID{out}...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}
	if inputAddr == outputAddr {
		sentValue = availableValue
	}

	// set balances
	outmap[outputAddr] = []*balance.Balance{balance.New(balance.ColorIOTA, sentValue)}
	outputs := transaction.NewOutputs(outmap)

	// handle remainder address
	if availableValue > sentValue {
		outputs.Add(inputAddr, []*balance.Balance{balance.New(balance.ColorIOTA, availableValue-sentValue)})
	}

	// sign transaction
	txn := transaction.New(inputs, outputs).Sign(sigScheme)

	// send transaction
	txId, err = from.SendTransaction(txn.Bytes())
	require.NoErrorf(t, err, "could not send transaction on %s", from.String())

	addrBalance[inputAddr.String()][balance.ColorIOTA] -= sentValue
	addrBalance[outputAddr.String()][balance.ColorIOTA] += sentValue

	return false, txId
}

// SendColoredTransactionOnRandomPeer sends colored tokens on a random peer, saves the sent token amount to a map, and returns transaction IDs.
func SendColoredTransactionOnRandomPeer(t *testing.T, peers []*framework.Peer, addrBalance map[string]map[balance.Color]int64, numMessages int) (txIds []string) {
	counter := 0
	for i := 0; i < numMessages; i++ {
		from := rand.Intn(len(peers))
		to := rand.Intn(len(peers))
		fail, txId := SendColoredTransaction(t, peers[from], peers[to], addrBalance)
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

// SendColoredTransaction sends IOTA and colored tokens from and to a given peer and returns the fail flag and the transaction ID.
// 1. Get the first unspent outputs of `from`
// 2. Accumulate the token amount of the first unspent output
// 3. Send 50 IOTA tokens + [accumalate token amount - 50] new minted tokens to `to`
func SendColoredTransaction(t *testing.T, from *framework.Peer, to *framework.Peer, addrBalance map[string]map[balance.Color]int64) (fail bool, txId string) {
	var sentValue int64 = 50
	var balanceList []*balance.Balance
	sigScheme := signaturescheme.ED25519(*from.Seed().KeyPair(0))
	inputAddr := from.Seed().Address(0)
	outputAddr := to.Seed().Address(0)

	// prepare inputs
	resp, err := from.GetUnspentOutputs([]string{inputAddr.String()})
	require.NoErrorf(t, err, "could not get unspent outputs on %s", from.String())

	// abort if no unspent outputs
	if len(resp.UnspentOutputs[0].OutputIDs) == 0 {
		return true, ""
	}

	// calculate available token in the unspent output
	var availableValue int64 = 0
	for _, b := range resp.UnspentOutputs[0].OutputIDs[0].Balances {
		availableValue += b.Value
		balanceList = append(balanceList, balance.New(getColorFromString(b.Color), (-1)*b.Value))
	}

	// abort if not enough tokens
	if availableValue < sentValue {
		return true, ""
	}

	out, err := transaction.OutputIDFromBase58(resp.UnspentOutputs[0].OutputIDs[0].ID)
	require.NoErrorf(t, err, "invalid unspent outputs ID on %s", from.String())
	inputs := transaction.NewInputs([]transaction.OutputID{out}...)

	// prepare outputs
	outmap := map[address.Address][]*balance.Balance{}

	// set balances
	outmap[outputAddr] = []*balance.Balance{balance.New(balance.ColorIOTA, sentValue)}
	if availableValue > sentValue {
		outmap[outputAddr] = append(outmap[outputAddr], balance.New(balance.ColorNew, availableValue-sentValue))
	}
	outputs := transaction.NewOutputs(outmap)

	// sign transaction
	txn := transaction.New(inputs, outputs).Sign(sigScheme)

	// send transaction
	txId, err = from.SendTransaction(txn.Bytes())
	require.NoErrorf(t, err, "could not send transaction on %s", from.String())

	// update balance list
	balanceList = append(balanceList, outmap[outputAddr]...)
	updateBalanceList(addrBalance, balanceList, inputAddr.String(), outputAddr.String(), txId)

	return false, txId
}

// updateBalanceList updates the token amount map with given peers and balances.
// If the value of balance is negative, it is the balance to be deducted from peer from, else it is deposited to peer to.
// If the color is balance.ColorNew, it should be recolored with txId.
func updateBalanceList(addrBalance map[string]map[balance.Color]int64, balances []*balance.Balance, from, to, txId string) {
	for _, b := range balances {
		color := b.Color()
		value := b.Value()
		if value < 0 {
			// deduct
			addrBalance[from][color] += value
			continue
		}
		// deposit
		if color == balance.ColorNew {
			addrBalance[to][getColorFromString(txId)] = value
			continue
		}
		addrBalance[to][color] += value
	}
	return
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

			// calculate the balances of each colored coin
			for _, unspents := range resp.UnspentOutputs[0].OutputIDs {
				for _, respBalance := range unspents.Balances {
					color := getColorFromString(respBalance.Color)
					sum[color] += respBalance.Value
				}
			}

			// check balances
			for color, value := range sum {
				assert.Equal(t, b[color], value)
			}
		}
	}
}

// CheckTransactions performs checks to make sure that all peers have received all transactions .
func CheckTransactions(t *testing.T, peers []*framework.Peer, transactionIDs []string, checkSynchronized bool) {
	for _, peer := range peers {
		if checkSynchronized {
			// check that the peer sees itself as synchronized
			info, err := peer.Info()
			require.NoError(t, err)
			require.True(t, info.Synced)
		}

		for _, txId := range transactionIDs {
			resp, err := peer.GetTransactionByID(txId)
			require.NoError(t, err)

			// check inclusion state
			assert.True(t, resp.InclusionState.Confirmed)
			assert.False(t, resp.InclusionState.Rejected)
		}
	}
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n Shutdowner) {
	err := n.Shutdown()
	require.NoError(t, err)
}
