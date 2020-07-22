package client

import (
	"errors"
	"net/http"

	webapi_tools "github.com/iotaledger/goshimmer/plugins/webapi/tools"
)

const (
	routePastCone = "tools/pastcone"
)

// PastConeExist checks that all of the messages in the past cone of a message are existing on the node
// down to the genesis. Returns the number of messages in the past cone as well.
func (api *GoShimmerAPI) PastConeExist(base58EncodedMessageID string) (*webapi_tools.PastConeResponse, error) {
	res := &webapi_tools.PastConeResponse{}

	if err := api.do(
		http.MethodGet,
		routePastCone,
		&webapi_tools.PastConeRequest{ID: base58EncodedMessageID},
		res,
	); err != nil {
		return nil, err
	}

	if res.Error != "" {
		return res, errors.New(res.Error)
	}
	return res, nil
}
