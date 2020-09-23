package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi"
)

const (
	routeInfo = "info"
)

// Info gets the info of the node.
func (api *GoShimmerAPI) Info() (*webapi.InfoResponse, error) {
	res := &webapi.InfoResponse{}
	if err := api.do(http.MethodGet, routeInfo, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
