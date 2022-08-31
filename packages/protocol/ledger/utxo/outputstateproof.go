package utxo

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/cerrors"
	"golang.org/x/crypto/blake2b"
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
		return errors.Errorf("proof index does not match output index: %w", cerrors.ErrFatal)
	}

	if err = o.OutputCommitmentProof.Validate(output); err != nil {
		return errors.Errorf("invalid output commitment proof: %w", err)
	}

	provenTxID := blake2b.Sum256(byteutils.ConcatBytes(o.TransactionCommitment[:], o.OutputCommitmentProof.OutputCommitment.Bytes()))
	if provenTxID != o.OutputID.TransactionID.Identifier {
		return errors.Errorf("invalid transaction ID: %w", cerrors.ErrFatal)
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
