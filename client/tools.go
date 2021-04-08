package client

import (
	"errors"
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routePastCone = "tools/message/pastcone"
	routeMissing  = "tools/message/missing"

	routeValueDebug = "tools/value/objects"
)

// ------------------- Communication layer -----------------------------

// PastConeExist checks that all of the messages in the past cone of a message are existing on the node
// down to the genesis. Returns the number of messages in the past cone as well.
func (api *GoShimmerAPI) PastConeExist(base58EncodedMessageID string) (*jsonmodels.PastconeResponse, error) {
	res := &jsonmodels.PastconeResponse{}

	if err := api.do(
		http.MethodGet,
		routePastCone,
		&jsonmodels.PastconeRequest{ID: base58EncodedMessageID},
		res,
	); err != nil {
		return nil, err
	}

	if res.Error != "" {
		return res, errors.New(res.Error)
	}
	return res, nil
}

// Missing returns all the missing messages and their count.
func (api *GoShimmerAPI) Missing() (*jsonmodels.MissingResponse, error) {
	res := &jsonmodels.MissingResponse{}
	if err := api.do(http.MethodGet, routeMissing, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ------------------- Value layer -----------------------------

// ValueObjects returns the list of value objects.
func (api *GoShimmerAPI) ValueObjects() (*jsonmodels.ObjectsResponse, error) {
	res := &jsonmodels.ObjectsResponse{}
	if err := api.do(http.MethodGet, routeValueDebug, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
