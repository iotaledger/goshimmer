package client

import (
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
	"net/http"
)

const (
	routeTransactionByID = "ledgerstate/transactions/:transactionID"
)

// GetTransactionByID returns the transaction, transaction metadata and inclusion state data based on transactionID
func (api *GoShimmerAPI) GetTransactionByID(base58EncodedID string) (*jsonmodels.GetTransactionByIDResponse, error) {
	res := &jsonmodels.GetTransactionByIDResponse{}

	if err := api.do(
		http.MethodGet,
		routeTransactionByID+base58EncodedID,
		nil,
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}
