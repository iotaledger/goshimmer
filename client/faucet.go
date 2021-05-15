package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/webapi/jsonmodels"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
)

const (
	routeFaucet = "faucet"
)

// SendFaucetRequest requests funds from faucet nodes by sending a faucet request payload message.
func (api *GoShimmerAPI) SendFaucetRequest(base58EncodedAddr string, pledgeIDs ...string) (*jsonmodels.FaucetResponse, error) {
	var aManaPledgeID identity.ID
	var cManaPledgeID identity.ID
	if len(pledgeIDs) > 1 {
		aManaPledgeIDFromString, err := mana.IDFromStr(pledgeIDs[0])
		if err != nil {
			aManaPledgeID = aManaPledgeIDFromString
		}
		cManaPledgeIDFromString, err := mana.IDFromStr(pledgeIDs[1])
		if err != nil {
			cManaPledgeID = cManaPledgeIDFromString
		}
	}

	// TODO: compute PoW

	res := &jsonmodels.FaucetResponse{}
	if err := api.do(http.MethodPost, routeFaucet,
		&jsonmodels.FaucetRequest{
			Address:               base58EncodedAddr,
			AccessManaPledgeID:    base58.Encode(aManaPledgeID.Bytes()),
			ConsensusManaPledgeID: base58.Encode(cManaPledgeID.Bytes()),
			Nonce:                 0,
		}, res); err != nil {
		return nil, err
	}

	return res, nil
}
