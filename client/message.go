package client

import (
	"net/http"

	webapi_message "github.com/iotaledger/goshimmer/plugins/webapi/message"
)

const (
	routeFindByID    = "message/findById"
	routeSendPayload = "message/sendPayload"
)

// FindMessageByID finds messages by the given base58 encoded IDs. The messages are returned in the same order as
// the given IDs. Non available messages are empty at their corresponding index.
func (api *GoShimmerAPI) FindMessageByID(base58EncodedIDs []string) (*webapi_message.Response, error) {
	res := &webapi_message.Response{}

	if err := api.do(
		http.MethodPost,
		routeFindByID,
		&webapi_message.Request{IDs: base58EncodedIDs},
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}

// SendPayload send a message with the given payload.
func (api *GoShimmerAPI) SendPayload(payload []byte) (string, error) {
	res := &webapi_message.MsgResponse{}
	if err := api.do(http.MethodPost, routeSendPayload,
		&webapi_message.MsgRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
