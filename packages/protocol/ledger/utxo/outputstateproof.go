package utxo

import (
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/cerrors"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
)

// region OutputStateProof /////////////////////////////////////////////////////////////////////////////////////////////

// OutputStateProof represents a cryptographic proof that a specific Output is the one named in the OutputID referenced
// in the proof.
type OutputStateProof struct {
	OutputID              OutputID
	TransactionCommitment TransactionCommitment
	OutputCommitmentProof *OutputCommitmentProof
}

// Validate validates the proof and checks if the given Output is indeed the one that is referenced in the proof.
func (o *OutputStateProof) Validate(output Output) (err error) {
	if uint64(o.OutputID.Index) != o.OutputCommitmentProof.ProofIndex {
		return errors.WithMessage(cerrors.ErrFatal, "proof index does not match output index")
	}

	if err = o.OutputCommitmentProof.Validate(output); err != nil {
		return errors.Wrap(err, "invalid output commitment proof")
	}

	provenTxID := blake2b.Sum256(byteutils.ConcatBytes(o.TransactionCommitment[:], o.OutputCommitmentProof.OutputCommitment.Bytes()))
	if provenTxID != o.OutputID.TransactionID.Identifier {
		return errors.WithMessage(cerrors.ErrFatal, "invalid transaction ID")
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
