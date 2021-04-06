package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels/value"
)

const (
	routeAttachments          = "value/attachments"
	routeGetTxnByID           = "value/transactionByID"
	routeSendTxn              = "value/sendTransaction"
	routeSendTxnByJSON        = "value/sendTransactionByJson"
	routeUnspentOutputs       = "value/unspentOutputs"
	routeAllowedPledgeNodeIDs = "value/allowedManaPledge"
)

// GetAttachments gets the attachments of a transaction ID
func (api *GoShimmerAPI) GetAttachments(base58EncodedTxnID string) (*value.AttachmentsResponse, error) {
	res := &value.AttachmentsResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeAttachments, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetTransactionByID gets the transaction of a transaction ID
func (api *GoShimmerAPI) GetTransactionByID(base58EncodedTxnID string) (*value.GetTransactionByIDResponse, error) {
	res := &value.GetTransactionByIDResponse{}
	if err := api.do(http.MethodGet, func() string {
		return fmt.Sprintf("%s?txnID=%s", routeGetTxnByID, base58EncodedTxnID)
	}(), nil, res); err != nil {
		return nil, err
	}

	return res, nil
}

// GetUnspentOutputs return unspent output IDs of addresses
func (api *GoShimmerAPI) GetUnspentOutputs(addresses []string) (*value.UnspentOutputsResponse, error) {
	res := &value.UnspentOutputsResponse{}
	if err := api.do(http.MethodPost, routeUnspentOutputs,
		&value.UnspentOutputsRequest{Addresses: addresses}, res); err != nil {
		return nil, err
	}

	return res, nil
}

// SendTransaction sends the transaction(bytes) to the Value Tangle and returns transaction ID.
func (api *GoShimmerAPI) SendTransaction(txnBytes []byte) (string, error) {
	res := &value.SendTransactionResponse{}
	if err := api.do(http.MethodPost, routeSendTxn,
		&value.SendTransactionRequest{TransactionBytes: txnBytes}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}

// SendTransactionByJSON sends the transaction(JSON) to the Value Tangle and returns transaction ID and message ID.
func (api *GoShimmerAPI) SendTransactionByJSON(txn value.SendTransactionByJSONRequest) (string, error) {
	res := &value.SendTransactionByJSONResponse{}
	if err := api.do(http.MethodPost, routeSendTxnByJSON,
		&value.SendTransactionByJSONRequest{
			Inputs:        txn.Inputs,
			Outputs:       txn.Outputs,
			AManaPledgeID: txn.AManaPledgeID,
			CManaPledgeID: txn.CManaPledgeID,
			Signatures:    txn.Signatures,
			Payload:       txn.Payload,
		}, res); err != nil {
		return "", err
	}

	return res.TransactionID, nil
}

// GetAllowedManaPledgeNodeIDs returns the list of allowed mana pledge IDs.
func (api *GoShimmerAPI) GetAllowedManaPledgeNodeIDs() (*value.AllowedManaPledgeResponse, error) {
	res := &value.AllowedManaPledgeResponse{}
	if err := api.do(http.MethodGet, routeAllowedPledgeNodeIDs, nil, res); err != nil {
		return nil, err
	}

	return res, nil
}
