package evilwallet

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2/serix"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// region utxo/tx realted functions ////////////////////////////////////////////////////////////////////////////////////////////

// SplitBalanceEqually splits the balance equally between `splitNumber` outputs.
func SplitBalanceEqually(splitNumber int, balance uint64) []uint64 {
	outputBalances := make([]uint64, 0)
	// make sure the output balances are equal input
	var totalBalance uint64 = 0
	// input is divided equally among outputs
	for i := 0; i < splitNumber-1; i++ {
		outputBalances = append(outputBalances, balance/uint64(splitNumber))
		totalBalance, _ = devnetvm.SafeAddUint64(totalBalance, outputBalances[i])
	}
	lastBalance, _ := devnetvm.SafeSubUint64(balance, totalBalance)
	outputBalances = append(outputBalances, lastBalance)

	return outputBalances
}

func getOutputIDsByJSON(outputs []*jsonmodels.Output) (outputIDs []utxo.OutputID) {
	for _, jsonOutput := range outputs {
		output, err := jsonOutput.ToLedgerstateOutput()
		if err != nil {
			continue
		}
		outputIDs = append(outputIDs, output.ID())
	}
	return outputIDs
}

func getOutputByJSON(jsonOutput *jsonmodels.Output) (output devnetvm.Output) {
	output, err := jsonOutput.ToLedgerstateOutput()
	if err != nil {
		return
	}

	return output
}

// RateSetterSleep sleeps for the given rate.
func RateSetterSleep(clt Client, useRateSetter bool) error {
	if useRateSetter {
		err := clt.SleepRateSetterEstimate()
		if err != nil {
			return err
		}
	}
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////

func SignBlock(block *models.Block, localID *identity.LocalIdentity) (ed25519.Signature, error) {
	contentHash, err := block.ContentHash()
	if err != nil {
		return ed25519.EmptySignature, errors.Wrap(err, "failed to obtain block content's hash")
	}
	issuingTimeBytes, err := serix.DefaultAPI.Encode(context.Background(), block.IssuingTime(), serix.WithValidation())
	if err != nil {
		return ed25519.EmptySignature, errors.Wrap(err, "failed to encode block issuing time")
	}
	commitmentIDBytes, err := block.Commitment().ID().Bytes()
	if err != nil {
		return ed25519.EmptySignature, errors.Wrap(err, "failed to encode block commitment")
	}
	return localID.Sign(byteutils.ConcatBytes(issuingTimeBytes, commitmentIDBytes, contentHash[:])), nil
}
