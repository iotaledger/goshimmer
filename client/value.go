package client

import (
	"fmt"
	"net/http"

	webapi_value "github.com/iotaledger/goshimmer/plugins/webapi/value"
)

const (
	routeAttachments    = "value/attachments"
	routeGetTxnByID     = "value/transactionByID"
	routeSendTxn        = "value/sendTransaction"
	routeSendTxnByJSON  = "value/sendTransactionByJson"
	routeUnspentOutputs = "value/unspentOutputs"
)

// GetAttachments gets the attachments of a transaction ID
func (api *GoShimmerAPI) GetAttachments(base58EncodedTxnID string) (*webapi_value.AttachmentsResponse, error) {
	res := &webapi_value.AttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeAttachments, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetTransactionByID gets the transaction of a transaction ID
func (api *GoShimmerAPI) GetTransactionByID(base58EncodedTxnID string) (*webapi_value.GetTransactionByIDResponse, error) {
	res := &webapi_value.GetTransactionByIDResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeGetTxnByID, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetUnspentOutputs return unspent output IDs of addresses
func (api *GoShimmerAPI) GetUnspentOutputs(addresses []string) (*webapi_value.UnspentOutputsResponse, error) {
	res := &webapi_value.UnspentOutputsResponse{}
	if err := api.do(http.MethodPost, routeUnspentOutputs,
		&webapi_value.UnspentOutputsRequest{Addresses: addresses}, res); err != nil {
		return nil, err
	}

	return res, nil
}

// SendTransaction sends the transaction(bytes) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransaction(txnBytes []byte) (string, error) {
	res := &webapi_value.SendTransactionResponse{}
	if err := api.do(http.MethodPost, routeSendTxn,
		&webapi_value.SendTransactionRequest{TransactionBytes: txnBytes}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}

// SendTransactionByJSON sends the transaction(JSON) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransactionByJSON(txn webapi_value.SendTransactionByJSONRequest) (string, error) {
	res := &webapi_value.SendTransactionByJSONResponse{}
	if err := api.do(http.MethodPost, routeSendTxnByJSON,
		&webapi_value.SendTransactionByJSONRequest{
			Inputs:     txn.Inputs,
			Outputs:    txn.Outputs,
			Data:       txn.Data,
			Signatures: txn.Signatures,
		}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}
