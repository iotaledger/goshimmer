package client

import (
	"net/http"

	webapi "github.com/iotaledger/goshimmer/plugins/webapi"
)

const (
	routeCollectiveBeacon = "drng/collectiveBeacon"
	routeRandomness       = "drng/info/randomness"
	routeCommittee        = "drng/info/committee"
)

// BroadcastCollectiveBeacon sends the given collective beacon (payload) by creating a message in the backend.
func (api *GoShimmerAPI) BroadcastCollectiveBeacon(payload []byte) (string, error) {

	res := &webapi.CollectiveBeaconResponse{}
	if err := api.do(http.MethodPost, routeCollectiveBeacon,
		&webapi.CollectiveBeaconRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}

// GetRandomness gets the current randomness.
func (api *GoShimmerAPI) GetRandomness() (*webapi.RandomnessResponse, error) {
	res := &webapi.RandomnessResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeRandomness
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetCommittee gets the current committee.
func (api *GoShimmerAPI) GetCommittee() (*webapi.CommitteeResponse, error) {
	res := &webapi.CommitteeResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeCommittee
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
