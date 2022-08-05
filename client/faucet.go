package client

import (
	"context"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/mana"
	"github.com/iotaledger/goshimmer/packages/core/pow"
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
		aManaPledgeIDFromString, err := mana.IDFromStr(pledgeIDs[0])
		if err == nil {
			aManaPledgeID = aManaPledgeIDFromString
		}
		cManaPledgeIDFromString, err := mana.IDFromStr(pledgeIDs[1])
		if err == nil {
			cManaPledgeID = cManaPledgeIDFromString
		}
	}

	address, err := devnetvm.AddressFromBase58EncodedString(base58EncodedAddr)
	if err != nil {
		return nil, errors.Errorf("could not decode address from string: %w", err)
	}

	nonce, err := computeFaucetPoW(address, aManaPledgeID, cManaPledgeID, powTarget)
	if err != nil {
		return nil, errors.Errorf("could not compute faucet PoW: %w", err)
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
		return nil, errors.Errorf("accessPledgeID and consensusPledgeID must not be empty")
	}
	aManaPledgeIDFromString, err := mana.IDFromStr(accessPledgeID)
	if err == nil {
		aManaPledgeID = aManaPledgeIDFromString
	}
	cManaPledgeIDFromString, err := mana.IDFromStr(consensusPledgeID)
	if err == nil {
		cManaPledgeID = cManaPledgeIDFromString
	}

	address, err := devnetvm.AddressFromBase58EncodedString(base58EncodedAddr)
	if err != nil {
		return nil, errors.Errorf("could not decode address from string: %w", err)
	}

	nonce, err := computeFaucetPoW(address, aManaPledgeID, cManaPledgeID, powTarget)
	if err != nil {
		return nil, errors.Errorf("could not compute faucet PoW: %w", err)
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
