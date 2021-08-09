package client

import (
	"context"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/faucet"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/pow"
)

const (
	routeFaucet = "faucet"
)

var (
	defaultPOWTarget = 25
	powWorker        = pow.New(1)
)

// SendFaucetRequest requests funds from faucet nodes by sending a faucet request payload message.
func (api *GoShimmerAPI) SendFaucetRequest(base58EncodedAddr string, powTarget int, pledgeIDs ...string) (*jsonmodels.FaucetResponse, error) {
	var aManaPledgeID identity.ID
	var cManaPledgeID identity.ID
	if len(pledgeIDs) > 1 {
		aManaPledgeIDFromString, err := mana.IDFromStr(pledgeIDs[0])
		if err == nil {
			aManaPledgeID = aManaPledgeIDFromString
		}
		cManaPledgeIDFromString, err := mana.IDFromStr(pledgeIDs[1])
		if err == nil {
			cManaPledgeID = cManaPledgeIDFromString
		}
	}

	address, err := ledgerstate.AddressFromBase58EncodedString(base58EncodedAddr)
	if err != nil {
		return nil, errors.Errorf("could not decode address from string: %w", err)
	}

	nonce, err := computeFaucetPoW(address, aManaPledgeID, cManaPledgeID, powTarget)
	if err != nil {
		return nil, errors.Errorf("could not compute faucet PoW: %w", err)
	}

	res := &jsonmodels.FaucetResponse{}
	if err := api.do(http.MethodPost, routeFaucet,
		&jsonmodels.FaucetRequest{
			Address:               base58EncodedAddr,
			AccessManaPledgeID:    base58.Encode(aManaPledgeID.Bytes()),
			ConsensusManaPledgeID: base58.Encode(cManaPledgeID.Bytes()),
			Nonce:                 nonce,
		}, res); err != nil {
		return nil, err
	}

	return res, nil
}

func computeFaucetPoW(address ledgerstate.Address, aManaPledgeID, cManaPledgeID identity.ID, powTarget int) (nonce uint64, err error) {
	if powTarget < 0 {
		powTarget = defaultPOWTarget
	}

	faucetRequest := faucet.NewRequest(address, aManaPledgeID, cManaPledgeID, 0)

	objectBytes := faucetRequest.Bytes()
	powRelevantBytes := objectBytes[:len(objectBytes)-pow.NonceBytes]

	return powWorker.Mine(context.Background(), powRelevantBytes, powTarget)
}
