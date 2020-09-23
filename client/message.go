package client

import (
	"net/http"

	webapi "github.com/iotaledger/goshimmer/plugins/webapi"
)

const (
	routeFindByID    = "message/findById"
	routeSendPayload = "message/sendPayload"
)

// FindMessageByID finds messages by the given base58 encoded IDs. The messages are returned in the same order as
// the given IDs. Non available messages are empty at their corresponding index.
func (api *GoShimmerAPI) FindMessageByID(base58EncodedIDs []string) (*webapi.FindByIDResponse, error) {
	res := &webapi.FindByIDResponse{}

	if err := api.do(
		http.MethodPost,
		routeFindByID,
		&webapi.FindByIDRequest{IDs: base58EncodedIDs},
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}

// SendPayload send a message with the given payload.
func (api *GoShimmerAPI) SendPayload(payload []byte) (string, error) {
	res := &webapi.MsgResponse{}
	if err := api.do(http.MethodPost, routeSendPayload,
		&webapi.MsgRequest{Payload: payload}, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
