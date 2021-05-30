package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
)

const (
	routeCollectiveBeacon = "drng/collectiveBeacon"
	routeRandomness       = "drng/info/randomness"
	routeCommittee        = "drng/info/committee"
)

// BroadcastCollectiveBeacon sends the given collective beacon (payload) by creating a message in the backend.
func (api *GoShimmerAPI) BroadcastCollectiveBeacon(payload []byte) (string, error) {
	res := &jsonmodels.CollectiveBeaconResponse{}
	if err := api.do(http.MethodPost, routeCollectiveBeacon,
		&jsonmodels.CollectiveBeaconRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}

// GetRandomness gets the current randomness.
func (api *GoShimmerAPI) GetRandomness() (*jsonmodels.RandomnessResponse, error) {
	res := &jsonmodels.RandomnessResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeRandomness
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetCommittee gets the current committee.
func (api *GoShimmerAPI) GetCommittee() (*jsonmodels.CommitteeResponse, error) {
	res := &jsonmodels.CommitteeResponse{}
	if err := api.do(http.MethodGet, func() string {
		return routeCommittee
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
