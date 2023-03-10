package client

import (
	"context"
	"net/http"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/pow"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/crypto/identity"
)

const (
	routeFaucetRequestBroadcast = "faucetrequest"
	routeFaucetRequestAPI       = "faucet"
)

var (
	defaultPOWTarget = 25
	powWorker        = pow.New(1)
)

// BroadcastFaucetRequest requests funds from faucet nodes by sending a faucet request payload block.
func (api *GoShimmerAPI) BroadcastFaucetRequest(base58EncodedAddr string, powTarget int, pledgeIDs ...string) (*jsonmodels.FaucetRequestResponse, error) {
	var aManaPledgeID identity.ID
	var cManaPledgeID identity.ID
	if len(pledgeIDs) > 1 {
		aManaPledgeIDFromString, err := identity.DecodeIDBase58(pledgeIDs[0])
		if err == nil {
			aManaPledgeID = aManaPledgeIDFromString
		}
		cManaPledgeIDFromString, err := identity.DecodeIDBase58(pledgeIDs[1])
		if err == nil {
			cManaPledgeID = cManaPledgeIDFromString
		}
	}

	address, err := devnetvm.AddressFromBase58EncodedString(base58EncodedAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not decode address from string")
	}

	nonce, err := computeFaucetPoW(address, aManaPledgeID, cManaPledgeID, powTarget)
	if err != nil {
		return nil, errors.Wrapf(err, "could not compute faucet PoW")
	}

	res := &jsonmodels.FaucetRequestResponse{}
	if err := api.do(http.MethodPost, routeFaucetRequestBroadcast,
		&jsonmodels.FaucetRequest{
			Address:               base58EncodedAddr,
			AccessManaPledgeID:    aManaPledgeID.EncodeBase58(),
			ConsensusManaPledgeID: cManaPledgeID.EncodeBase58(),
			Nonce:                 nonce,
		}, res); err != nil {
		return nil, err
	}

	return res, nil
}

// SendFaucetRequestAPI requests funds from faucet nodes by sending a faucet request directly to the faucet node.
func (api *GoShimmerAPI) SendFaucetRequestAPI(base58EncodedAddr string, powTarget int, accessPledgeID, consensusPledgeID string) (*jsonmodels.FaucetAPIResponse, error) {
	var aManaPledgeID identity.ID
	var cManaPledgeID identity.ID
	if accessPledgeID == "" && consensusPledgeID == "" {
		return nil, errors.New("accessPledgeID and consensusPledgeID must not be empty")
	}
	aManaPledgeIDFromString, err := identity.DecodeIDBase58(accessPledgeID)
	if err == nil {
		aManaPledgeID = aManaPledgeIDFromString
	}
	cManaPledgeIDFromString, err := identity.DecodeIDBase58(consensusPledgeID)
	if err == nil {
		cManaPledgeID = cManaPledgeIDFromString
	}

	address, err := devnetvm.AddressFromBase58EncodedString(base58EncodedAddr)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode address from string")
	}

	nonce, err := computeFaucetPoW(address, aManaPledgeID, cManaPledgeID, powTarget)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute faucet PoW")
	}

	res := &jsonmodels.FaucetAPIResponse{}
	if err := api.do(http.MethodPost, routeFaucetRequestAPI,
		&jsonmodels.FaucetRequest{
			Address:               base58EncodedAddr,
			AccessManaPledgeID:    aManaPledgeID.EncodeBase58(),
			ConsensusManaPledgeID: cManaPledgeID.EncodeBase58(),
			Nonce:                 nonce,
		}, res); err != nil {
		return nil, err
	}

	return res, nil
}

func computeFaucetPoW(address devnetvm.Address, aManaPledgeID, cManaPledgeID identity.ID, powTarget int) (nonce uint64, err error) {
	if powTarget < 0 {
		powTarget = defaultPOWTarget
	}

	faucetRequest := faucet.NewRequest(address, aManaPledgeID, cManaPledgeID, 0)

	objectBytes, err := faucetRequest.Bytes()
	if err != nil {
		return
	}
	powRelevantBytes := objectBytes[:len(objectBytes)-pow.NonceBytes]

	return powWorker.Mine(context.Background(), powRelevantBytes, powTarget)
}
