package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routeMessage         = "messages/"
	routeMessageMetadata = "/metadata"
	routeSendPayload     = "messages/payload"
)

// GetMessage is the handler for the /messages/:messageID endpoint.
func (api *GoShimmerAPI) GetMessage(base58EncodedID string) (*jsonmodels.Message, error) {
	res := &jsonmodels.Message{}

	if err := api.do(
		http.MethodGet,
		routeMessage+base58EncodedID,
		nil,
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}

// GetMessageMetadata is the handler for the /messages/:messageID/metadata endpoint.
func (api *GoShimmerAPI) GetMessageMetadata(base58EncodedID string) (*jsonmodels.MessageMetadata, error) {
	res := &jsonmodels.MessageMetadata{}

	if err := api.do(
		http.MethodGet,
		routeMessage+base58EncodedID+routeMessageMetadata,
		nil,
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}

// SendPayload send a message with the given payload.
func (api *GoShimmerAPI) SendPayload(payload []byte) (string, error) {
	res := &jsonmodels.PostPayloadResponse{}
	if err := api.do(http.MethodPost, routeSendPayload,
		&jsonmodels.PostPayloadRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
