package evilwallet

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/cockroachdb/errors"
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

func getIotaColorAmount(balance *devnetvm.ColoredBalances) uint64 {
	outBalance := uint64(0)
	balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
		if color == devnetvm.ColorIOTA {
			outBalance += balance
		}
		return true
	})
	return outBalance
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

func SignBlock(block *models.Block, privKey ed25519.PrivateKey) (ed25519.Signature, error) {
	contentHash, err := block.ContentHash()
	if err != nil {
		return ed25519.EmptySignature, errors.Wrap(err, "failed to obtain block content's hash")
	}
	issuingTimeBytes, err := serix.DefaultAPI.Encode(context.Background(), block.IssuingTime(), serix.WithValidation())
	if err != nil {
		return ed25519.EmptySignature, errors.Wrap(err, "failed to encode block issuing time")
	}
	b, err := block.Commitment().Bytes()
	if err != nil {
		return ed25519.EmptySignature, errors.Wrap(err, "failed to encode block commitment")
	}
	return privKey.Sign(byteutils.ConcatBytes(issuingTimeBytes, b, contentHash[:])), nil
}

// region commitment utils  /////////////////////////////////////////////////////////////////////////////////////////////////////////////

// todo improve dumy commitments
func DummyCommitment(clt Client) (comm *commitment.Commitment, latestCinfIndex epoch.Index, err error) {
	latestCommitmentResp, err := clt.GetLatestCommitment()
	latestCommitment := new(commitment.Commitment)
	_, err = latestCommitment.FromBytes(latestCommitmentResp.Bytes)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to parse the bytes of the latest commitment")
	}
	return latestCommitment, epoch.Index(latestCommitmentResp.Index), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////
