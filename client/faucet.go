package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routeFaucet = "faucet"
)

// SendFaucetRequest requests funds from faucet nodes by sending a faucet request payload message.
func (api *GoShimmerAPI) SendFaucetRequest(base58EncodedAddr string) (*jsonmodels.FaucetResponse, error) {
	res := &jsonmodels.FaucetResponse{}
	if err := api.do(http.MethodPost, routeFaucet,
		&jsonmodels.FaucetRequest{Address: base58EncodedAddr}, res); err != nil {
		return nil, err
	}

	return res, nil
}
