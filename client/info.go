package client

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"
)

const (
	routeInfo = "info"
)

// Info gets the info of the node.
func (api *GoShimmerAPI) Info() (*jsonmodels2.InfoResponse, error) {
	res := &jsonmodels2.InfoResponse{}
	if err := api.do(http.MethodGet, routeInfo, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
