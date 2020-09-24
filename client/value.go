package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

const (
	routeAttachments    = "value/attachments"
	routeGetTxnByID     = "value/transactionByID"
	routeSendTxn        = "value/sendTransaction"
	routeSendTxnByJSON  = "value/sendTransactionByJson"
	routeUnspentOutputs = "value/unspentOutputs"
)

// GetAttachments gets the attachments of a transaction ID
func (api *GoShimmerAPI) GetAttachments(base58EncodedTxnID string) (*webapi.AttachmentsResponse, error) {
	res := &webapi.AttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeAttachments, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetTransactionByID gets the transaction of a transaction ID
func (api *GoShimmerAPI) GetTransactionByID(base58EncodedTxnID string) (*webapi.GetTransactionByIDResponse, error) {
	res := &webapi.GetTransactionByIDResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeGetTxnByID, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetUnspentOutputs return unspent output IDs of addresses
func (api *GoShimmerAPI) GetUnspentOutputs(addresses []string) (*webapi.UnspentOutputsResponse, error) {
	res := &webapi.UnspentOutputsResponse{}
	if err := api.do(http.MethodPost, routeUnspentOutputs,
		&webapi.UnspentOutputsRequest{Addresses: addresses}, res); err != nil {
		return nil, err
	}

	return res, nil
}

// SendTransaction sends the transaction(bytes) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransaction(txnBytes []byte) (string, error) {
	res := &webapi.SendTransactionResponse{}
	if err := api.do(http.MethodPost, routeSendTxn,
		&webapi.SendTransactionRequest{TransactionBytes: txnBytes}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}

// SendTransactionByJSON sends the transaction(JSON) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransactionByJSON(txn webapi.SendTxByJSONRequest) (string, error) {
	res := &webapi.SendTxByJSONResponse{}
	if err := api.do(http.MethodPost, routeSendTxnByJSON,
		&webapi.SendTxByJSONRequest{
			Inputs:     txn.Inputs,
			Outputs:    txn.Outputs,
			Data:       txn.Data,
			Signatures: txn.Signatures,
		}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}
