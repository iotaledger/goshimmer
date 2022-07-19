package client

import (
	"net/http"
	"strings"

	jsonmodels2 "github.com/iotaledger/goshimmer/packages/app/jsonmodels"
)

const (
	// basic routes.
	routeGetAddresses     = "ledgerstate/addresses/"
	routeGetConflicts     = "ledgerstate/conflicts/"
	routeGetOutputs       = "ledgerstate/outputs/"
	routeGetTransactions  = "ledgerstate/transactions/"
	routePostTransactions = "ledgerstate/transactions"

	// route path modifiers.
	pathUnspentOutputs = "/unspentOutputs"
	pathChildren       = "/children"
	pathConflicts      = "/conflicts"
	pathConsumers      = "/consumers"
	pathMetadata       = "/metadata"
	pathVoters         = "/voters"
	pathAttachments    = "/attachments"
)

// GetAddressOutputs gets the spent and unspent outputs of an address.
func (api *GoShimmerAPI) GetAddressOutputs(base58EncodedAddress string) (*jsonmodels2.GetAddressResponse, error) {
	res := &jsonmodels2.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetAddresses, base58EncodedAddress}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetAddressUnspentOutputs gets the unspent outputs of an address.
func (api *GoShimmerAPI) GetAddressUnspentOutputs(base58EncodedAddress string) (*jsonmodels2.GetAddressResponse, error) {
	res := &jsonmodels2.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetAddresses, base58EncodedAddress, pathUnspentOutputs}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// PostAddressUnspentOutputs gets the unspent outputs of several addresses.
func (api *GoShimmerAPI) PostAddressUnspentOutputs(base58EncodedAddresses []string) (*jsonmodels2.PostAddressesUnspentOutputsResponse, error) {
	res := &jsonmodels2.PostAddressesUnspentOutputsResponse{}
	if err := api.do(http.MethodPost, func() string {
		return strings.Join([]string{routeGetAddresses, "unspentOutputs"}, "")
	}(), &jsonmodels2.PostAddressesUnspentOutputsRequest{Addresses: base58EncodedAddresses}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflict gets the conflict information.
func (api *GoShimmerAPI) GetConflict(base58EncodedConflictID string) (*jsonmodels2.Conflict, error) {
	res := &jsonmodels2.Conflict{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetConflicts, base58EncodedConflictID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflictChildren gets the children of a conflict.
func (api *GoShimmerAPI) GetConflictChildren(base58EncodedConflictID string) (*jsonmodels2.GetConflictChildrenResponse, error) {
	res := &jsonmodels2.GetConflictChildrenResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetConflicts, base58EncodedConflictID, pathChildren}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflictConflicts gets the conflict conflicts of a conflict.
func (api *GoShimmerAPI) GetConflictConflicts(base58EncodedConflictID string) (*jsonmodels2.GetConflictConflictsResponse, error) {
	res := &jsonmodels2.GetConflictConflictsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetConflicts, base58EncodedConflictID, pathConflicts}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflictVoters gets the Voters of a conflict.
func (api *GoShimmerAPI) GetConflictVoters(base58EncodedConflictID string) (*jsonmodels2.GetConflictVotersResponse, error) {
	res := &jsonmodels2.GetConflictVotersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetConflicts, base58EncodedConflictID, pathVoters}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutput gets the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutput(base58EncodedOutputID string) (*jsonmodels2.Output, error) {
	res := &jsonmodels2.Output{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputConsumers gets the consumers of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputConsumers(base58EncodedOutputID string) (*jsonmodels2.GetOutputConsumersResponse, error) {
	res := &jsonmodels2.GetOutputConsumersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID, pathConsumers}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputMetadata gets the metadata of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputMetadata(base58EncodedOutputID string) (*jsonmodels2.OutputMetadata, error) {
	res := &jsonmodels2.OutputMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID, pathMetadata}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransaction gets the transaction of the corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransaction(base58EncodedTransactionID string) (*jsonmodels2.Transaction, error) {
	res := &jsonmodels2.Transaction{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionMetadata gets metadata of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionMetadata(base58EncodedTransactionID string) (*jsonmodels2.TransactionMetadata, error) {
	res := &jsonmodels2.TransactionMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathMetadata}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionAttachments gets the attachments (blockIDs) of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionAttachments(base58EncodedTransactionID string) (*jsonmodels2.GetTransactionAttachmentsResponse, error) {
	res := &jsonmodels2.GetTransactionAttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathAttachments}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// PostTransaction sends the transaction(bytes) to the Tangle and returns its transaction ID.
func (api *GoShimmerAPI) PostTransaction(transactionBytes []byte) (*jsonmodels2.PostTransactionResponse, error) {
	res := &jsonmodels2.PostTransactionResponse{}
	if err := api.do(http.MethodPost, routePostTransactions,
		&jsonmodels2.PostTransactionRequest{TransactionBytes: transactionBytes}, res); err != nil {
		return nil, err
	}

	return res, nil
}
