package client

import (
	"net/http"
	"strings"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	// basic routes.
	routeGetAddresses     = "ledgerstate/addresses/"
	routeGetBranches      = "ledgerstate/branches/"
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
		return strings.Join([]string{routeGetAddresses, base58EncodedAddress}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetAddressUnspentOutputs gets the unspent outputs of an address.
func (api *GoShimmerAPI) GetAddressUnspentOutputs(base58EncodedAddress string) (*jsonmodels.GetAddressResponse, error) {
	res := &jsonmodels.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetAddresses, base58EncodedAddress, pathUnspentOutputs}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// PostAddressUnspentOutputs gets the unspent outputs of several addresses.
func (api *GoShimmerAPI) PostAddressUnspentOutputs(base58EncodedAddresses []string) (*jsonmodels.PostAddressesUnspentOutputsResponse, error) {
	res := &jsonmodels.PostAddressesUnspentOutputsResponse{}
	if err := api.do(http.MethodPost, func() string {
		return strings.Join([]string{routeGetAddresses, "unspentOutputs"}, "")
	}(), &jsonmodels.PostAddressesUnspentOutputsRequest{Addresses: base58EncodedAddresses}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranch gets the branch information.
func (api *GoShimmerAPI) GetBranch(base58EncodedBranchID string) (*jsonmodels.Branch, error) {
	res := &jsonmodels.Branch{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranchChildren gets the children of a branch.
func (api *GoShimmerAPI) GetBranchChildren(base58EncodedBranchID string) (*jsonmodels.GetBranchChildrenResponse, error) {
	res := &jsonmodels.GetBranchChildrenResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID, pathChildren}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranchConflicts gets the conflict branches of a branch.
func (api *GoShimmerAPI) GetBranchConflicts(base58EncodedBranchID string) (*jsonmodels.GetBranchConflictsResponse, error) {
	res := &jsonmodels.GetBranchConflictsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID, pathConflicts}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranchVoters gets the Voters of a branch.
func (api *GoShimmerAPI) GetBranchVoters(base58EncodedBranchID string) (*jsonmodels.GetBranchVotersResponse, error) {
	res := &jsonmodels.GetBranchVotersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID, pathVoters}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutput gets the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutput(base58EncodedOutputID string) (*jsonmodels.Output, error) {
	res := &jsonmodels.Output{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputConsumers gets the consumers of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputConsumers(base58EncodedOutputID string) (*jsonmodels.GetOutputConsumersResponse, error) {
	res := &jsonmodels.GetOutputConsumersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID, pathConsumers}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputMetadata gets the metadata of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputMetadata(base58EncodedOutputID string) (*jsonmodels.OutputMetadata, error) {
	res := &jsonmodels.OutputMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID, pathMetadata}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransaction gets the transaction of the corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransaction(base58EncodedTransactionID string) (*jsonmodels.Transaction, error) {
	res := &jsonmodels.Transaction{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionMetadata gets metadata of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionMetadata(base58EncodedTransactionID string) (*jsonmodels.TransactionMetadata, error) {
	res := &jsonmodels.TransactionMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathMetadata}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionAttachments gets the attachments (messageIDs) of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionAttachments(base58EncodedTransactionID string) (*jsonmodels.GetTransactionAttachmentsResponse, error) {
	res := &jsonmodels.GetTransactionAttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathAttachments}, "")
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
