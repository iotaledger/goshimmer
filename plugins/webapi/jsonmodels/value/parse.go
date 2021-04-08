package value

import (
	"github.com/mr-tron/base58/base58"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// ParseTransaction handle transaction json object.
func ParseTransaction(tx *ledgerstate.Transaction) (txn Transaction) {
	// process inputs
	inputs := make([]Input, len(tx.Essence().Inputs()))
	for i, input := range tx.Essence().Inputs() {
		inputs[i] = Input{
			ConsumedOutputID: input.Base58(),
		}
	}

	// process outputs
	outputs := make([]Output, len(tx.Essence().Outputs()))
	for i, output := range tx.Essence().Outputs() {
		var balances []Balance
		output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
			balances = append(balances, Balance{
				Color: color.String(),
				Value: int64(balance),
			})
			return true
		})
		outputs[i] = Output{
			Type:     output.Type(),
			Address:  output.Address().Base58(),
			Balances: balances,
		}
	}

	// process unlock blocks
	unlockBlocks := make([]UnlockBlock, len(tx.UnlockBlocks()))
	for i, unlockBlock := range tx.UnlockBlocks() {
		ub := UnlockBlock{
			Type: unlockBlock.Type(),
		}
		switch unlockBlock.Type() {
		case ledgerstate.SignatureUnlockBlockType:
			signature, _, _ := ledgerstate.SignatureFromBytes(unlockBlock.Bytes())
			ub.SignatureType = signature.Type()
			switch signature.Type() {
			case ledgerstate.ED25519SignatureType:
				signature, _, _ := ledgerstate.ED25519SignatureFromBytes(signature.Bytes())
				ub.PublicKey = signature.PublicKey.String()
				ub.Signature = signature.Signature.String()

			case ledgerstate.BLSSignatureType:
				signature, _, _ := ledgerstate.BLSSignatureFromBytes(signature.Bytes())
				ub.Signature = signature.Signature.String()
			}
		case ledgerstate.ReferenceUnlockBlockType:
			referenceUnlockBlock, _, _ := ledgerstate.ReferenceUnlockBlockFromBytes(unlockBlock.Bytes())
			ub.ReferencedIndex = referenceUnlockBlock.ReferencedIndex()
		}

		unlockBlocks[i] = ub
	}

	dataPayload := []byte{}
	if tx.Essence().Payload() != nil {
		dataPayload = tx.Essence().Payload().Bytes()
	}

	return Transaction{
		Version:           tx.Essence().Version(),
		Timestamp:         tx.Essence().Timestamp().Unix(),
		AccessPledgeID:    base58.Encode(tx.Essence().AccessPledgeID().Bytes()),
		ConsensusPledgeID: base58.Encode(tx.Essence().ConsensusPledgeID().Bytes()),
		Inputs:            inputs,
		Outputs:           outputs,
		UnlockBlocks:      unlockBlocks,
		DataPayload:       dataPayload,
	}
}
