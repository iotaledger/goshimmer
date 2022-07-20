package client

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"

	"github.com/iotaledger/goshimmer/packages/core/markers"
)

const (
	// basic routes.
	routeGetMarkers = "weightprovider/markers/"

	// route path modifiers.
	pathWeight = "/weight"
)

// GetMarkerVoters gets the Voters of a marker.
func (api *GoShimmerAPI) GetMarkerVoters(sequenceID markers.SequenceID, markerIndex markers.Index) (*jsonmodels.MarkerVotersResponse, error) {
	res := &jsonmodels.MarkerVotersResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetMarkers, strconv.FormatUint(uint64(sequenceID), 10), "/", strconv.FormatUint(uint64(markerIndex), 10), pathVoters}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// GetMarkerWeight gets the weight of a marker.
func (api *GoShimmerAPI) GetMarkerWeight(sequenceID markers.SequenceID, markerIndex markers.Index) (*jsonmodels.MarkerWeightResponse, error) {
	res := &jsonmodels.MarkerWeightResponse{}
	if err := api.do(http.MethodGet, func() string {
		return strings.Join([]string{routeGetMarkers, strconv.FormatUint(uint64(sequenceID), 10), "/", strconv.FormatUint(uint64(markerIndex), 10), pathWeight}, "")
	}(), nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
