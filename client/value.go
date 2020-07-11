package client

import (
	"fmt"
	"net/http"

	webapi_attachments "github.com/iotaledger/goshimmer/plugins/webapi/value/attachments"
	webapi_gettxn "github.com/iotaledger/goshimmer/plugins/webapi/value/gettransactionbyid"
	webapi_sendtxn "github.com/iotaledger/goshimmer/plugins/webapi/value/sendtransaction"
	webapi_sendtxnbyjson "github.com/iotaledger/goshimmer/plugins/webapi/value/sendtransactionbyjson"
	webapi_unspentoutputs "github.com/iotaledger/goshimmer/plugins/webapi/value/unspentoutputs"
)

const (
	routeAttachments    = "value/attachments"
	routeGetTxnByID     = "value/transactionByID"
	routeSendTxn        = "value/sendTransaction"
	routeSendTxnByJSON  = "value/sendTransactionByJson"
	routeUnspentOutputs = "value/unspentOutputs"
)

// GetAttachments gets the attachments of a transaction ID
func (api *GoShimmerAPI) GetAttachments(base58EncodedTxnID string) (*webapi_attachments.Response, error) {
	res := &webapi_attachments.Response{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeAttachments, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetTransactionByID gets the transaction of a transaction ID
func (api *GoShimmerAPI) GetTransactionByID(base58EncodedTxnID string) (*webapi_gettxn.Response, error) {
	res := &webapi_gettxn.Response{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeGetTxnByID, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetUnspentOutputs return unspent output IDs of addresses
func (api *GoShimmerAPI) GetUnspentOutputs(addresses []string) (*webapi_unspentoutputs.Response, error) {
	res := &webapi_unspentoutputs.Response{}
	if err := api.do(http.MethodPost, routeUnspentOutputs,
		&webapi_unspentoutputs.Request{Addresses: addresses}, res); err != nil {
		return nil, err
	}

	return res, nil
}

// SendTransaction sends the transaction(bytes) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransaction(txnBytes []byte) (string, error) {
	res := &webapi_sendtxn.Response{}
	if err := api.do(http.MethodPost, routeSendTxn,
		&webapi_sendtxn.Request{TransactionBytes: txnBytes}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}

// SendTransactionByJSON sends the transaction(JSON) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransactionByJSON(txn webapi_sendtxnbyjson.Request) (string, error) {
	res := &webapi_sendtxn.Response{}
	if err := api.do(http.MethodPost, routeSendTxnByJSON,
		&webapi_sendtxnbyjson.Request{
			Inputs:     txn.Inputs,
			Outputs:    txn.Outputs,
			Data:       txn.Data,
			Signatures: txn.Signatures,
		}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}
