package wallet

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/lo"
)

// WebConnector implements a connector that uses the web API to connect to a node to implement the required functions
// for the wallet.
type WebConnector struct {
	client *client.GoShimmerAPI
}

// NewWebConnector is the constructor for the WebConnector.
func NewWebConnector(baseURL string, setters ...client.Option) *WebConnector {
	return &WebConnector{
		client: client.NewGoShimmerAPI(baseURL, setters...),
	}
}

// ServerStatus retrieves the connected server status with Info api.
func (webConnector *WebConnector) ServerStatus() (status ServerStatus, err error) {
	response, err := webConnector.client.Info()
	if err != nil {
		return
	}

	status.ID = response.IdentityID
	status.Synced = response.TangleTime.Synced
	status.Version = response.Version

	return
}

// RequestFaucetFunds request some funds from the faucet for test purposes.
func (webConnector *WebConnector) RequestFaucetFunds(addr address.Address, powTarget int) (err error) {
	err = webConnector.client.SleepRateSetterEstimate()
	if err != nil {
		return err
	}

	_, err = webConnector.client.BroadcastFaucetRequest(addr.Address().Base58(), powTarget)

	return
}

// UnspentOutputs returns the outputs of transactions on the given addresses that have not been spent yet.
func (webConnector WebConnector) UnspentOutputs(addresses ...address.Address) (unspentOutputs OutputsByAddressAndOutputID, err error) {
	// build reverse lookup table + arguments for client call
	addressReverseLookupTable := make(map[string]address.Address)
	base58EncodedAddresses := make([]string, len(addresses))
	for i, addr := range addresses {
		base58EncodedAddresses[i] = addr.Address().Base58()
		addressReverseLookupTable[addr.Address().Base58()] = addr
	}

	// request unspent outputs
	response, err := webConnector.client.PostAddressUnspentOutputs(base58EncodedAddresses)
	if err != nil {
		return
	}

	// build result
	unspentOutputs = make(map[address.Address]map[utxo.OutputID]*Output)
	for _, unspentOutput := range response.UnspentOutputs {
		// lookup wallet address from raw address
		addr, addressRequested := addressReverseLookupTable[unspentOutput.Address.Base58]
		if !addressRequested {
			panic("the server returned an unrequested address")
		}

		// iterate through outputs
		for _, output := range unspentOutput.Outputs {
			lOutput, err := output.Output.ToLedgerstateOutput()
			if err != nil {
				return nil, err
			}
			// build output
			walletOutput := &Output{
				Address:                  addr,
				Object:                   lOutput,
				ConfirmationStateReached: output.ConfirmationState.IsAccepted(),
				Spent:                    false,
				Metadata: OutputMetadata{
					Timestamp: output.Metadata.Timestamp,
				},
			}

			// store output in result
			if _, addressExists := unspentOutputs[addr]; !addressExists {
				unspentOutputs[addr] = make(map[utxo.OutputID]*Output)
			}
			unspentOutputs[addr][walletOutput.Object.ID()] = walletOutput
		}
	}

	return
}

// SendTransaction sends a new transaction to the network.
func (webConnector WebConnector) SendTransaction(tx *devnetvm.Transaction) (err error) {
	err = webConnector.client.SleepRateSetterEstimate()
	if err != nil {
		return err
	}
	txBytes, err := tx.Bytes()
	if err != nil {
		return err
	}
	_, err = webConnector.client.PostTransaction(txBytes)

	return
}

// GetTransactionConfirmationState fetches the ConfirmationState of the transaction.
func (webConnector WebConnector) GetTransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State, err error) {
	txmeta, err := webConnector.client.GetTransactionMetadata(txID.Base58())
	if err != nil {
		return
	}
	return txmeta.ConfirmationState, nil
}

// GetUnspentAliasOutput returns the current unspent alias output that belongs to a given alias address.
func (webConnector WebConnector) GetUnspentAliasOutput(addr *devnetvm.AliasAddress) (output *devnetvm.AliasOutput, err error) {
	res, err := webConnector.client.GetAddressOutputs(addr.Base58())
	if err != nil {
		return
	}
	for _, o := range res.UnspentOutputs {
		if o.Type != devnetvm.AliasOutputType.String() {
			continue
		}
		var uncastedOutput devnetvm.Output
		uncastedOutput, err = o.ToLedgerstateOutput()
		if err != nil {
			return
		}
		alias, ok := uncastedOutput.(*devnetvm.AliasOutput)
		if !ok {
			err = errors.New("alias output received from api cannot be casted to ledgerstate representation")
			return
		}
		if alias.GetAliasAddress().Equals(addr) {
			// we found what we were looking for
			output = alias
			return
		}
	}
	return nil, errors.Errorf("couldn't find unspent alias output for alias addr %s", addr.Base58())
}

// colorFromString is an internal utility method that parses the given string into a Color.
func colorFromString(colorStr string) (color devnetvm.Color) {
	if colorStr == "IOTA" {
		color = devnetvm.ColorIOTA
	} else {
		var t utxo.TransactionID
		_ = t.FromBase58(colorStr)
		color, _, _ = devnetvm.ColorFromBytes(lo.PanicOnErr(t.Bytes()))
	}
	return
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ Connector = &WebConnector{}
