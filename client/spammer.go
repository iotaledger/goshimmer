package client

import (
	"fmt"
	"net/http"

	webapi_spammer "github.com/iotaledger/goshimmer/plugins/spammer"
)

const (
	routeSpammer = "spammer"
)

// ToggleSpammer toggles the node internal spammer.
func (api *GoShimmerAPI) ToggleSpammer(enable bool) (*webapi_spammer.Response, error) {
	res := &webapi_spammer.Response{}
	if err := api.do(http.MethodGet, func() string {
		if enable {
			return fmt.Sprintf("%s?cmd=start", routeSpammer)
		}
		return fmt.Sprintf("%s?cmd=stop", routeSpammer)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
