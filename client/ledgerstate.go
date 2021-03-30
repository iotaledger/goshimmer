package client

import (
	"net/http"
	"strings"

	json_models "github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routeGetUnspentOutputPt1 = "ledgerstate/addresses/"
	routeGetUnspentOutputPt2 = "/unspentOutputs"
)

// GetAttachments gets the attachments of a transaction ID
func (api *GoShimmerAPI) GetAddressUnspentOutputs(base58EncodedAddress string) (*json_models.GetAddressResponse, error) {
	res := &json_models.GetAddressResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetUnspentOutputPt1, base58EncodedAddress, routeGetUnspentOutputPt2}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
