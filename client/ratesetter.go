package client

import (
	"net/http"
	"time"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
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

// SleepRateSetterEstimate gets the rate-setter estimate and the rate-setter info and later sleeps the estimated amount of time.
func (api *GoShimmerAPI) SleepRateSetterEstimate() error {
	res, err := api.RateSetter()
	if err != nil {
		return err
	}
	time.Sleep(res.Estimate)
	return nil
}
