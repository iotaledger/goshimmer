package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

const (
	routeData = "data"
)

// Data sends the given data (payload) by creating a message in the backend.
func (api *GoShimmerAPI) Data(data []byte) (string, error) {

	res := &webapi.DataResponse{}
	if err := api.do(http.MethodPost, routeData,
		&webapi.DataRequest{Data: data}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
