package client

import (
	"errors"
	"net/http"

	webapi_tools_message "github.com/iotaledger/goshimmer/plugins/webapi/tools/message"
	webapi_tools_value "github.com/iotaledger/goshimmer/plugins/webapi/tools/value"
)

const (
	routePastCone = "tools/message/pastcone"
	routeMissing  = "tools/message/missing"

	routeValueTips  = "tools/value/tips"
	routeValueDebug = "tools/value/objects"
)

// ------------------- Communication layer -----------------------------

// PastConeExist checks that all of the messages in the past cone of a message are existing on the node
// down to the genesis. Returns the number of messages in the past cone as well.
func (api *GoShimmerAPI) PastConeExist(base58EncodedMessageID string) (*webapi_tools_message.PastconeResponse, error) {
	res := &webapi_tools_message.PastconeResponse{}

	if err := api.do(
		http.MethodGet,
		routePastCone,
		&webapi_tools_message.PastconeRequest{ID: base58EncodedMessageID},
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
func (api *GoShimmerAPI) Missing() (*webapi_tools_message.MissingResponse, error) {
	res := &webapi_tools_message.MissingResponse{}
	if err := api.do(http.MethodGet, routeMissing, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ------------------- Value layer -----------------------------

// ValueTips returns the value objects info from the tips.
func (api *GoShimmerAPI) ValueTips() (*webapi_tools_value.TipsResponse, error) {
	res := &webapi_tools_value.TipsResponse{}
	if err := api.do(http.MethodGet, routeValueTips, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ValueObjects returns the list of value objects.
func (api *GoShimmerAPI) ValueObjects() (*webapi_tools_value.ObjectsResponse, error) {
	res := &webapi_tools_value.ObjectsResponse{}
	if err := api.do(http.MethodGet, routeValueDebug, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
