package client

import (
	"net/http"
	"strings"

	json_models "github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	// basic routes
	routeGetAddresses    = "ledgerstate/addresses/"
	routeGetBranches     = "ledgerstate/branches/"
	routeGetOutputs      = "ledgerstate/outputs/"
	routeGetTransactions = "ledgerstate/transactions/"

	// route path modifiers
	pathUnspentOutputs = "/unspentOutputs"
	pathChildren       = "/children"
	pathConflicts      = "/conflicts"
	pathConsumers      = "/consumers"
	pathMetadata       = "/metadata"
	pathInclusionState = "/inclusionState"
	pathConsensus      = "/consensus"
	pathAttachments    = "/attachments"
)

// GetAddressOutputs gets the spent and unspent outputs of an address.
func (api *GoShimmerAPI) GetAddressOutputs(base58EncodedAddress string) (*json_models.GetAddressResponse, error) {
	res := &json_models.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetAddresses, base58EncodedAddress}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetAddressUnspentOutputs gets the unspent outputs of an address.
func (api *GoShimmerAPI) GetAddressUnspentOutputs(base58EncodedAddress string) (*json_models.GetAddressResponse, error) {
	res := &json_models.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetAddresses, base58EncodedAddress, pathUnspentOutputs}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// PostAddressUnspentOutputs gets the unspent outputs of several addresses.
func (api *GoShimmerAPI) PostAddressUnspentOutputs(base58EncodedAddresses []string) (*json_models.PostAddressesUnspentOutputsResponse, error) {
	res := &json_models.PostAddressesUnspentOutputsResponse{}
	if err := api.do(http.MethodPost, func() string {
		return strings.Join([]string{routeGetAddresses, "unspentOutputs"}, "")
	}(), &json_models.PostAddressesUnspentOutputsRequest{Addresses: base58EncodedAddresses}, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranch gets the branch information.
func (api *GoShimmerAPI) GetBranch(base58EncodedBranchID string) (*json_models.Branch, error) {
	res := &json_models.Branch{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranchChildren gets the children of a branch.
func (api *GoShimmerAPI) GetBranchChildren(base58EncodedBranchID string) (*json_models.GetBranchChildrenResponse, error) {
	res := &json_models.GetBranchChildrenResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID, pathChildren}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetBranchConflicts gets the conflict branches of a branch.
func (api *GoShimmerAPI) GetBranchConflicts(base58EncodedBranchID string) (*json_models.GetBranchConflictsResponse, error) {
	res := &json_models.GetBranchConflictsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetBranches, base58EncodedBranchID, pathConflicts}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutput gets the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutput(base58EncodedOutputID string) (*json_models.Output, error) {
	res := &json_models.Output{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputConsumers gets the consumers of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputConsumers(base58EncodedOutputID string) (*json_models.GetOutputConsumersResponse, error) {
	res := &json_models.GetOutputConsumersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID, pathConsumers}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetOutputMetadata gets the metadata of the output corresponding to OutputID.
func (api *GoShimmerAPI) GetOutputMetadata(base58EncodedOutputID string) (*json_models.OutputMetadata, error) {
	res := &json_models.OutputMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetOutputs, base58EncodedOutputID, pathMetadata}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransaction gets the transaction of the corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransaction(base58EncodedTransactionID string) (*json_models.Transaction, error) {
	//	res := &jsonmodels.GetTransactionByIDResponse{}
	res := &json_models.Transaction{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionMetadata gets metadata of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionMetadata(base58EncodedTransactionID string) (*json_models.TransactionMetadata, error) {
	res := &json_models.TransactionMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathMetadata}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionInclusionState gets inclusion state of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionInclusionState(base58EncodedTransactionID string) (*json_models.TransactionInclusionState, error) {
	res := &json_models.TransactionInclusionState{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathInclusionState}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionConsensusMetadata gets the consensus metadata of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionConsensusMetadata(base58EncodedTransactionID string) (*json_models.TransactionConsensusMetadata, error) {
	res := &json_models.TransactionConsensusMetadata{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathConsensus}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetTransactionAttachments gets the attachments (messageIDs) of the transaction corresponding to TransactionID.
func (api *GoShimmerAPI) GetTransactionAttachments(base58EncodedTransactionID string) (*json_models.GetTransactionAttachmentsResponse, error) {
	res := &json_models.GetTransactionAttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetTransactions, base58EncodedTransactionID, pathAttachments}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
