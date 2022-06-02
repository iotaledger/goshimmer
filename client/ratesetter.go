package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	rateSetterInfo = "ratesetter"
)

// RateSetter gets the ratesetter estimate and the rate-setter info.
func (api *GoShimmerAPI) RateSetter() (*jsonmodels.RateSetter, error) {
	res := &jsonmodels.RateSetter{}
	if err := api.do(http.MethodGet, rateSetterInfo, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
