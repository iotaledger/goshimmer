package client

import (
	"net/http"

	webapi_message "github.com/iotaledger/goshimmer/plugins/webapi/message"
)

const (
	routeFindById = "message/findById"
)

// FindMessageById finds messages by the given ids. The messages are returned in the same order as
// the given ids. Non available messages are empty at their corresponding index.
func (api *GoShimmerAPI) FindMessageById(base58EncodedIds []string) (*webapi_message.Response, error) {
	res := &webapi_message.Response{}

	if err := api.do(
		http.MethodPost,
		routeFindById,
		&webapi_message.Request{Ids: base58EncodedIds},
		res,
	); err != nil {
		return nil, err
	}

	return res, nil
}
