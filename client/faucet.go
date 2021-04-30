package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
)

const (
	routeFaucet = "faucet"
)

// SendFaucetRequest requests funds from faucet nodes by sending a faucet request payload message.
func (api *GoShimmerAPI) SendFaucetRequest(base58EncodedAddr string, pledgeIDs ...string) (*jsonmodels.FaucetResponse, error) {
	aManaPledgeID, cManaPledgeID := "", ""
	if len(pledgeIDs) > 1 {
		aManaPledgeID, cManaPledgeID = pledgeIDs[0], pledgeIDs[1]
	}
	res := &jsonmodels.FaucetResponse{}
	if err := api.do(http.MethodPost, routeFaucet,
		&jsonmodels.FaucetRequest{Address: base58EncodedAddr, AccessManaPledgeID: aManaPledgeID, ConsensusManaPledgeID: cManaPledgeID}, res); err != nil {
		return nil, err
	}

	return res, nil
}
