package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routeSpammer = "spammer"
)

// ToggleSpammer toggles the node internal spammer.
func (api *GoShimmerAPI) ToggleSpammer(enable bool, mpm int) (*jsonmodels.SpammerResponse, error) {
	res := &jsonmodels.SpammerResponse{}
	if err := api.do(http.MethodGet, func() string {
		if enable {
			return fmt.Sprintf("%s?cmd=start&mpm=%d", routeSpammer, mpm)
		}
		return fmt.Sprintf("%s?cmd=stop", routeSpammer)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
