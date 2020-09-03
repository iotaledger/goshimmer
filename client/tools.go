package client

import (
	"errors"
	"net/http"

	webapi_missing "github.com/iotaledger/goshimmer/plugins/webapi/tools/message/missing"
	webapi_pastcone "github.com/iotaledger/goshimmer/plugins/webapi/tools/message/pastcone"
	webapi_value_objects "github.com/iotaledger/goshimmer/plugins/webapi/tools/value/objects"
	webapi_value_tips "github.com/iotaledger/goshimmer/plugins/webapi/tools/value/tips"
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
func (api *GoShimmerAPI) PastConeExist(base58EncodedMessageID string) (*webapi_pastcone.Response, error) {
	res := &webapi_pastcone.Response{}

	if err := api.do(
		http.MethodGet,
		routePastCone,
		&webapi_pastcone.Request{ID: base58EncodedMessageID},
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
func (api *GoShimmerAPI) Missing() (*webapi_missing.Response, error) {
	res := &webapi_missing.Response{}
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

// ValueObjects returns the list of value objects.
func (api *GoShimmerAPI) ValueObjects() (*webapi_value_objects.Response, error) {
	res := &webapi_value_objects.Response{}
	if err := api.do(http.MethodGet, routeValueDebug, nil, res); err != nil {
		return nil, err
	}
	return res, nil
}
