package client

import (
	"net/http"

	webapi_message "github.com/iotaledger/goshimmer/plugins/webapi/message"
)

const (
	routeFindById = "findById"
)

func (api *GoShimmerAPI) FindMessageById(base58EncodedIds []string) (*webapi_message.Response, error) {
	res := &webapi_message.Response{}

	err := api.do(
		http.MethodPost,
		routeFindById,
		&webapi_message.Request{Ids: base58EncodedIds},
		res,
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}
