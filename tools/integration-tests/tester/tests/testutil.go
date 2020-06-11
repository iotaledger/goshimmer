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
		}
	}
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n Shutdowner) {
	err := n.Shutdown()
	require.NoError(t, err)
}
