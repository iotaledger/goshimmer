package client

import (
	jsonmodels2 "github.com/iotaledger/goshimmer/packages/jsonmodels"
	"net/http"

	"github.com/cockroachdb/errors"
)

const (
	routePastCone = "tools/message/pastcone"
	routeMissing  = "tools/message/missing"
)

// ------------------- Communication layer -----------------------------

// PastConeExist checks that all of the messages in the past cone of a message are existing on the node
// down to the genesis. Returns the number of messages in the past cone as well.
func (api *GoShimmerAPI) PastConeExist(base58EncodedMessageID string) (*jsonmodels2.PastconeResponse, error) {
	res := &jsonmodels2.PastconeResponse{}

	if err := api.do(
		http.MethodGet,
		routePastCone,
		&jsonmodels2.PastconeRequest{ID: base58EncodedMessageID},
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
func (api *GoShimmerAPI) Missing() (*jsonmodels2.MissingResponse, error) {
	res := &jsonmodels2.MissingResponse{}
	if err := api.do(http.MethodGet, routeMissing, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
