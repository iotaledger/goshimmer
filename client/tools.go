package client

import (
	"errors"
	"net/http"

	webapi_tools "github.com/iotaledger/goshimmer/plugins/webapi/tools"
)

const (
	routePastCone         = "tools/pastcone"
	routeMessageApprovers = "tools/messageapprovers"
	routeValueApprovers   = "tools/valueapprovers"
)

// ValueApprovers returns the approvers of the specified payloadID.
func (api *GoShimmerAPI) ValueApprovers(base58EncodedPayloadID string) (*webapi_tools.ValueApproversResponse, error) {
	res := &webapi_tools.ValueApproversResponse{}
	if err := api.do(
		http.MethodGet,
		routeValueApprovers,
		&webapi_tools.ValueApproversRequest{ID: base58EncodedPayloadID},
		res,
	); err != nil {
		return nil, err
	}

	if res.Error != "" {
		return res, errors.New(res.Error)
	}
	return res, nil
}

// MessageApprovers returns the approvers of the specified messageID.
func (api *GoShimmerAPI) MessageApprovers(base58EncodedMessageID string) (*webapi_tools.MessageApproversResponse, error) {
	res := &webapi_tools.MessageApproversResponse{}
	if err := api.do(
		http.MethodGet,
		routeMessageApprovers,
		&webapi_tools.MessageApproversRequest{ID: base58EncodedMessageID},
		res,
	); err != nil {
		return nil, err
	}

	if res.Error != "" {
		return res, errors.New(res.Error)
	}
	return res, nil
}

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
