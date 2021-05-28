package client

import (
	"fmt"
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"
)

const (
	routeSpammer = "spammer"
)

// ToggleSpammer toggles the node internal spammer.
func (api *GoShimmerAPI) ToggleSpammer(enable bool, mpm int) (*jsonmodels2.SpammerResponse, error) {
	res := &jsonmodels2.SpammerResponse{}
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
