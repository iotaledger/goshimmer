package client

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"
)

const (
	routeData = "data"
)

// Data sends the given data (payload) by creating a message in the backend.
func (api *GoShimmerAPI) Data(data []byte) (string, error) {
	res := &jsonmodels2.DataResponse{}
	if err := api.do(http.MethodPost, routeData,
		&jsonmodels2.DataRequest{Data: data}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
