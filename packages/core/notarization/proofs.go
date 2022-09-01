package notarization

import (
	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// region proofs helpers ///////////////////////////////////////////////////////////////////////////////////////////////

// CommitmentProof represents an inclusion proof for a specific epoch.
type CommitmentProof struct {
	EI    epoch.Index
	proof smt.SparseMerkleProof
	root  []byte
}

// GetBlockInclusionProof gets the proof of the inclusion (acceptance) of a block.
func (m *Manager) GetBlockInclusionProof(blockID tangleold.BlockID) (*CommitmentProof, error) {
	var ei epoch.Index
	m.tangle.Storage.Block(blockID).Consume(func(block *tangleold.Block) {
		t := block.IssuingTime()
		ei = epoch.IndexFromTime(t)
	})
	proof, err := m.epochCommitmentFactory.ProofTangleRoot(ei, blockID)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// GetTransactionInclusionProof gets the proof of the inclusion (acceptance) of a transaction.
func (m *Manager) GetTransactionInclusionProof(transactionID utxo.TransactionID) (*CommitmentProof, error) {
	var ei epoch.Index
	m.tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(txMeta *ledger.TransactionMetadata) {
		ei = epoch.IndexFromTime(txMeta.InclusionTime())
	})
	proof, err := m.epochCommitmentFactory.ProofStateMutationRoot(ei, transactionID)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

func (f *EpochCommitmentFactory) verifyRoot(proof CommitmentProof, key []byte, value []byte) bool {
	return smt.VerifyProof(proof.proof, proof.root, key, value, lo.PanicOnErr(blake2b.New256(nil)))
}

// ProofStateRoot returns the merkle proof for the outputID against the state root.
func (f *EpochCommitmentFactory) ProofStateRoot(ei epoch.Index, outID utxo.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root, exists := f.commitmentTrees.Get(ei)
	if !exists {
		return nil, errors.Errorf("could not obtain commitment trees for epoch %d", ei)
	}
	tangleRoot := root.tangleTree.Root()
	proof, err := f.stateRootTree.ProveForRoot(key, tangleRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, tangleRoot}, nil
}

// ProofStateMutationRoot returns the merkle proof for the transactionID against the state mutation root.
func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei epoch.Index, txID utxo.TransactionID) (*CommitmentProof, error) {
	committmentTrees, err := f.getCommitmentTrees(ei)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get commitment trees for epoch %d", ei)
	}

	key := txID.Bytes()
	root := committmentTrees.stateMutationTree.Root()
	proof, err := committmentTrees.stateMutationTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state mutation root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofTangleRoot returns the merkle proof for the blockID against the tangle root.
func (f *EpochCommitmentFactory) ProofTangleRoot(ei epoch.Index, blockID tangleold.BlockID) (*CommitmentProof, error) {
	committmentTrees, err := f.getCommitmentTrees(ei)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get commitment trees for epoch %d", ei)
	}

	key := blockID.Bytes()
	root := committmentTrees.tangleTree.Root()
	proof, err := committmentTrees.tangleTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the tangle root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// VerifyTangleRoot verify the provided merkle proof against the tangle root.
func (f *EpochCommitmentFactory) VerifyTangleRoot(proof CommitmentProof, blockID tangleold.BlockID) bool {
	key := blockID.Bytes()
	return f.verifyRoot(proof, key, key)
}

// VerifyStateMutationRoot verify the provided merkle proof against the state mutation root.
func (f *EpochCommitmentFactory) VerifyStateMutationRoot(proof CommitmentProof, transactionID utxo.TransactionID) bool {
	key := transactionID.Bytes()
	return f.verifyRoot(proof, key, key)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
