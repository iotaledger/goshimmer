package client

import (
	"net/http"

	webapi_message "github.com/iotaledger/goshimmer/plugins/webapi/message"
)

const (
	routeMessage         = "messages/"
	routeMessageMetadata = "/metadata"
	routeSendPayload     = "messages/sendPayload"
)

// GetMessage is the handler for the /messages/:messageID endpoint.
func (api *GoShimmerAPI) GetMessage(base58EncodedID string) (*webapi_message.Message, error) {
	res := &webapi_message.Message{}

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
func (api *GoShimmerAPI) GetMessageMetadata(base58EncodedID string) (*webapi_message.MessageMetadata, error) {
	res := &webapi_message.MessageMetadata{}

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
	res := &webapi_message.SendPayloadResponse{}
	if err := api.do(http.MethodPost, routeSendPayload,
		&webapi_message.SendPayloadRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
