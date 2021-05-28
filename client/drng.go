package client

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"
)

const (
	routeCollectiveBeacon = "drng/collectiveBeacon"
	routeRandomness       = "drng/info/randomness"
	routeCommittee        = "drng/info/committee"
)

// BroadcastCollectiveBeacon sends the given collective beacon (payload) by creating a message in the backend.
func (api *GoShimmerAPI) BroadcastCollectiveBeacon(payload []byte) (string, error) {
	res := &jsonmodels2.CollectiveBeaconResponse{}
	if err := api.do(http.MethodPost, routeCollectiveBeacon,
		&jsonmodels2.CollectiveBeaconRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}

// GetRandomness gets the current randomness.
func (api *GoShimmerAPI) GetRandomness() (*jsonmodels2.RandomnessResponse, error) {
	res := &jsonmodels2.RandomnessResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeRandomness
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetCommittee gets the current committee.
func (api *GoShimmerAPI) GetCommittee() (*jsonmodels2.CommitteeResponse, error) {
	res := &jsonmodels2.CommitteeResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeCommittee
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
