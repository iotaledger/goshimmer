package client

import (
	"net/http"

	webapi_data "github.com/iotaledger/goshimmer/plugins/webapi/data"
)

const (
	routeData = "data"
)

// Data sends the given data (payload) by creating a message in the backend.
func (api *GoShimmerAPI) Data(data []byte) (string, error) {

	res := &webapi_data.Response{}
	if err := api.do(http.MethodPost, routeData,
		&webapi_data.Request{Data: data}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
