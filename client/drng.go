package client

import (
	"net/http"

	webapi_drng "github.com/iotaledger/goshimmer/plugins/webapi/drng"
	webapi_drng_info "github.com/iotaledger/goshimmer/plugins/webapi/drng/info"
)

const (
	routeCollectiveBeacon = "drng/collectiveBeacon"
	routeRandomness       = "drng/info/randomness"
	routeCommittee        = "drng/info/committee"
)

// BroadcastCollectiveBeacon sends the given collective beacon (payload) by creating a message in the backend.
func (api *GoShimmerAPI) BroadcastCollectiveBeacon(payload []byte) (string, error) {

	res := &webapi_drng.CollectiveBeaconResponse{}
	if err := api.do(http.MethodPost, routeCollectiveBeacon,
		&webapi_drng.CollectiveBeaconRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}

// GetRandomness gets the current randomness.
func (api *GoShimmerAPI) GetRandomness() (*webapi_drng_info.RandomnessResponse, error) {
	res := &webapi_drng_info.RandomnessResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeRandomness
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetCommittee gets the current committee.
func (api *GoShimmerAPI) GetCommittee() (*webapi_drng_info.CommitteeResponse, error) {
	res := &webapi_drng_info.CommitteeResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeCommittee
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
