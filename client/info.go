package client

import (
	"net/http"

	webapi_info "github.com/iotaledger/goshimmer/plugins/webapi/info"
)

const (
	routeInfo = "info"
)

// Info gets the info of the node.
func (api *GoShimmerAPI) Info(knownPeers bool) (*webapi_info.Response, error) {
	res := &webapi_info.Response{}
	if err := api.do(http.MethodGet, func() string {
		return routeInfo
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
