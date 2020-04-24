package client

import (
	"net/http"

	webapi_collectiveBeacon "github.com/iotaledger/goshimmer/plugins/webapi/drng/collectivebeacon"
	webapi_committee "github.com/iotaledger/goshimmer/plugins/webapi/drng/info/committee"
	webapi_randomness "github.com/iotaledger/goshimmer/plugins/webapi/drng/info/randomness"
)

const (
	routeCollectiveBeacon = "drng/collectiveBeacon"
	routeRandomness       = "drng/info/randomness"
	routeCommittee        = "drng/info/committee"
)

// BroadcastCollectiveBeacon sends the given collective beacon (payload) by creating a message in the backend.
func (api *GoShimmerAPI) BroadcastCollectiveBeacon(payload []byte) (string, error) {

	res := &webapi_collectiveBeacon.Response{}
	if err := api.do(http.MethodPost, routeCollectiveBeacon,
		&webapi_collectiveBeacon.Request{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}

// GetRandomness gets the current randomness.
func (api *GoShimmerAPI) GetRandomness() (*webapi_randomness.Response, error) {
	res := &webapi_randomness.Response{}
	if err := api.do(http.MethodGet, func() string {
		return routeRandomness
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetCommittee gets the current committee.
func (api *GoShimmerAPI) GetCommittee() (*webapi_committee.Response, error) {
	res := &webapi_committee.Response{}
	if err := api.do(http.MethodGet, func() string {
		return routeCommittee
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
