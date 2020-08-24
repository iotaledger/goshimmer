package client

import (
	"errors"
	"net/http"

	webapi_missing "github.com/iotaledger/goshimmer/plugins/webapi/tools/message/missing"
	webapi_pastcone "github.com/iotaledger/goshimmer/plugins/webapi/tools/message/pastcone"
	webapi_value_debug "github.com/iotaledger/goshimmer/plugins/webapi/tools/value/debug"
	webapi_value_tips "github.com/iotaledger/goshimmer/plugins/webapi/tools/value/tips"
)

const (
	routePastCone = "tools/message/pastcone"
	routeMissing  = "tools/message/missing"

	routeValueTips  = "tools/value/tips"
	routeValueDebug = "tools/value/debug"
)

// ------------------- Communication layer -----------------------------

// PastConeExist checks that all of the messages in the past cone of a message are existing on the node
// down to the genesis. Returns the number of messages in the past cone as well.
func (api *GoShimmerAPI) PastConeExist(base58EncodedMessageID string) (*webapi_pastcone.PastConeResponse, error) {
	res := &webapi_pastcone.PastConeResponse{}

	if err := api.do(
		http.MethodGet,
		routePastCone,
		&webapi_pastcone.PastConeRequest{ID: base58EncodedMessageID},
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
func (api *GoShimmerAPI) Missing() (*webapi_missing.MissingResponse, error) {
	res := &webapi_missing.MissingResponse{}
	if err := api.do(http.MethodGet, routeMissing, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ------------------- Value layer -----------------------------

// ValueTips returns the value objects info from the tips.
func (api *GoShimmerAPI) ValueTips() (*webapi_value_tips.Response, error) {
	res := &webapi_value_tips.Response{}
	if err := api.do(http.MethodGet, routeValueTips, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ValueDebug returns returns the list of value objects.
func (api *GoShimmerAPI) ValueDebug() (*webapi_value_debug.Response, error) {
	res := &webapi_value_debug.Response{}
	if err := api.do(http.MethodGet, routeValueDebug, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
