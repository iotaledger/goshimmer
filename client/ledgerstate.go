package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
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
func (api *GoShimmerAPI) GetAddressOutputs(base58EncodedAddress string) (*jsonmodels.GetAddressResponse, error) {
	res := &jsonmodels.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetAddresses + base58EncodedAddress
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// PostAddressUnspentOutputs gets the unspent outputs of several addresses.
func (api *GoShimmerAPI) PostAddressUnspentOutputs(base58EncodedAddresses []string) (*jsonmodels.PostAddressesUnspentOutputsResponse, error) {
	res := &jsonmodels.PostAddressesUnspentOutputsResponse{}
	if err := api.do(http.MethodPost, func() string {
		return routeGetAddresses + "unspentOutputs"
	}(), &jsonmodels.PostAddressesUnspentOutputsRequest{Addresses: base58EncodedAddresses}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflict gets the conflict information.
func (api *GoShimmerAPI) GetConflict(base58EncodedConflictID string) (*jsonmodels.Conflict, error) {
	res := &jsonmodels.Conflict{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetConflicts + base58EncodedConflictID
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflictChildren gets the children of a conflict.
func (api *GoShimmerAPI) GetConflictChildren(base58EncodedConflictID string) (*jsonmodels.GetConflictChildrenResponse, error) {
	res := &jsonmodels.GetConflictChildrenResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetConflicts + base58EncodedConflictID + pathChildren
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflictConflicts gets the conflict conflicts of a conflict.
func (api *GoShimmerAPI) GetConflictConflicts(base58EncodedConflictID string) (*jsonmodels.GetConflictConflictsResponse, error) {
	res := &jsonmodels.GetConflictConflictsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetConflicts + base58EncodedConflictID + pathConflicts
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetConflictVoters gets the Voters of a conflict.
func (api *GoShimmerAPI) GetConflictVoters(base58EncodedConflictID string) (*jsonmodels.GetConflictVotersResponse, error) {
	res := &jsonmodels.GetConflictVotersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetConflicts + base58EncodedConflictID + pathVoters
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutput gets the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutput(base58EncodedOutputID string) (*jsonmodels.Output, error) {
	res := &jsonmodels.Output{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetOutputs + base58EncodedOutputID
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputConsumers gets the consumers of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputConsumers(base58EncodedOutputID string) (*jsonmodels.GetOutputConsumersResponse, error) {
	res := &jsonmodels.GetOutputConsumersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetOutputs + base58EncodedOutputID + pathConsumers
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputMetadata gets the metadata of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputMetadata(base58EncodedOutputID string) (*jsonmodels.OutputMetadata, error) {
	res := &jsonmodels.OutputMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetOutputs + base58EncodedOutputID + pathMetadata
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransaction gets the transaction of the corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransaction(base58EncodedTransactionID string) (*jsonmodels.Transaction, error) {
	res := &jsonmodels.Transaction{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetTransactions + base58EncodedTransactionID
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionMetadata gets metadata of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionMetadata(base58EncodedTransactionID string) (*jsonmodels.TransactionMetadata, error) {
	res := &jsonmodels.TransactionMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetTransactions + base58EncodedTransactionID + pathMetadata
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionAttachments gets the attachments (blockIDs) of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionAttachments(base58EncodedTransactionID string) (*jsonmodels.GetTransactionAttachmentsResponse, error) {
	res := &jsonmodels.GetTransactionAttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeGetTransactions + base58EncodedTransactionID + pathAttachments
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// PostTransaction sends the transaction(bytes) to the Tangle and returns its transaction ID.
func (api *GoShimmerAPI) PostTransaction(transactionBytes []byte) (*jsonmodels.PostTransactionResponse, error) {
	res := &jsonmodels.PostTransactionResponse{}
	if err := api.do(http.MethodPost, routePostTransactions,
		&jsonmodels.PostTransactionRequest{TransactionBytes: transactionBytes}, res); err != nil {
		return nil, err
	}

	return res, nil
}
