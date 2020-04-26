package client

import (
	"net/http"

	webapi_message "github.com/iotaledger/goshimmer/plugins/webapi/message"
)

const (
	routeFindByID = "message/findById"
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
