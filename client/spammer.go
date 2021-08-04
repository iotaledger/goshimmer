package client

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	routeSpammer = "spammer"
)

// ToggleSpammer toggles the node internal spammer.
func (api *GoShimmerAPI) ToggleSpammer(enable bool, mps int, imif string) (*jsonmodels.SpammerResponse, error) {
	// set default imif in case of incorrect imif value
	if imif != "poisson" && imif != "uniform" {
		imif = "uniform"
	}
	res := &jsonmodels.SpammerResponse{}
	if err := api.do(http.MethodGet, func() string {
		if enable {
			return fmt.Sprintf("%s?cmd=start&mps=%d&imif=%s", routeSpammer, mps, imif)
		}
		return fmt.Sprintf("%s?cmd=stop", routeSpammer)
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
