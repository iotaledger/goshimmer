package client

import (
	"net/http"

	webapi_faucet "github.com/iotaledger/goshimmer/plugins/webapi/faucet"
)

const (
	routeFaucet = "faucet"
)

// SendFaucetRequest requests funds from faucet nodes by sending a faucet request payload message.
func (api *GoShimmerAPI) SendFaucetRequest(base58EncodedAddr string) (*webapi_faucet.Response, error) {
	res := &webapi_faucet.Response{}
	if err := api.do(http.MethodPost, routeFaucet,
		&webapi_faucet.Request{Address: base58EncodedAddr}, res); err != nil {
		return nil, err
	}

	return res, nil
}
