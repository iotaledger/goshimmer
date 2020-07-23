package wallet

import (
	"net/http"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
)

// WebConnector implements a connector that uses the web API to connect to a node to implement the required functions
// for the wallet.
type WebConnector struct {
	client *client.GoShimmerAPI
}

// NewWebConnector is the constructor for the WebConnector.
func NewWebConnector(baseURL string, httpClient ...http.Client) *WebConnector {
	return &WebConnector{
		client: client.NewGoShimmerAPI(baseURL, httpClient...),
	}
}

// ServerStatus retrieves the connected server status with Info api.
func (webConnector *WebConnector) ServerStatus() (status ServerStatus, err error) {
	response, err := webConnector.client.Info()
	if err != nil {
		return
	}

	status.ID = response.IdentityID
	status.Synced = response.Synced
	status.Version = response.Version

	return
}

// RequestFaucetFunds request some funds from the faucet for test purposes.
func (webConnector *WebConnector) RequestFaucetFunds(addr wallet.Address) (err error) {
	_, err = webConnector.client.SendFaucetRequest(addr.String())

	return
}

// UnspentOutputs returns the outputs of transactions on the given addresses that have not been spent yet.
func (webConnector WebConnector) UnspentOutputs(addresses ...wallet.Address) (unspentOutputs map[wallet.Address]map[transaction.ID]*wallet.Output, err error) {
	// build reverse lookup table + arguments for client call
	addressReverseLookupTable := make(map[string]wallet.Address)
	base58EncodedAddresses := make([]string, len(addresses))
	for i, addr := range addresses {
		base58EncodedAddresses[i] = addr.String()
		addressReverseLookupTable[addr.String()] = addr
	}

	// request unspent outputs
	response, err := webConnector.client.GetUnspentOutputs(base58EncodedAddresses)
	if err != nil {
		return
	}

	// build result
	unspentOutputs = make(map[wallet.Address]map[transaction.ID]*wallet.Output)
	for _, unspentOutput := range response.UnspentOutputs {
		// lookup wallet address from raw address
		addr, addressRequested := addressReverseLookupTable[unspentOutput.Address]
		if !addressRequested {
			panic("the server returned an unrequested address")
		}

		// iterate through outputs
		for _, output := range unspentOutput.OutputIDs {
			// parse output id
			outputID, parseErr := transaction.OutputIDFromBase58(output.ID)
			if parseErr != nil {
				err = parseErr

				return
			}

			// build balances map
			balancesByColor := make(map[balance.Color]uint64)
			for _, bal := range output.Balances {
				color := colorFromString(bal.Color)
				balancesByColor[color] += uint64(bal.Value)
			}

			// build output
			walletOutput := &wallet.Output{
				Address:       addr.Address,
				TransactionID: outputID.TransactionID(),
				Balances:      balancesByColor,
				InclusionState: wallet.InclusionState{
					Liked:       output.InclusionState.Liked,
					Confirmed:   output.InclusionState.Confirmed,
					Rejected:    output.InclusionState.Rejected,
					Conflicting: output.InclusionState.Conflicting,
					Spent:       false,
				},
			}

			// store output in result
			if _, addressExists := unspentOutputs[addr]; !addressExists {
				unspentOutputs[addr] = make(map[transaction.ID]*wallet.Output)
			}
			unspentOutputs[addr][walletOutput.TransactionID] = walletOutput
		}
	}

	return
}

// SendTransaction sends a new transaction to the network.
func (webConnector WebConnector) SendTransaction(tx *transaction.Transaction) (err error) {
	_, err = webConnector.client.SendTransaction(tx.Bytes())

	return
}

// colorFromString is an internal utility method that parses the given string into a Color.
func colorFromString(colorStr string) (color balance.Color) {
	if colorStr == "IOTA" {
		color = balance.ColorIOTA
	} else {
		t, _ := transaction.IDFromBase58(colorStr)
		color, _, _ = balance.ColorFromBytes(t.Bytes())
	}
	return
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ wallet.Connector = &WebConnector{}
