package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	routeInfo = "info"
)

// Info gets the info of the node.
func (api *GoShimmerAPI) Info() (*jsonmodels.InfoResponse, error) {
	res := &jsonmodels.InfoResponse{}
	if err := api.do(http.MethodGet, routeInfo, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
