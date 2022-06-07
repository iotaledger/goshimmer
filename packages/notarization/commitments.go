package notarization

import (
	"hash"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/celestiaorg/smt"
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	ECCreationMaxDepth = 10
)

type Commitment struct {
	EI                epoch.EI
	tangleRoot        epoch.MerkleRoot
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	prevECR           *epoch.ECR
}

// CommitmentTrees is a compressed form of all the information (messages and confirmed value payloads) of an epoch.
type CommitmentTrees struct {
	EI                epoch.EI
	tangleTree        *smt.SparseMerkleTree
	stateMutationTree *smt.SparseMerkleTree
	prevECR           *epoch.ECR
}

// TangleRoot returns the root of the tangle sparse merkle tree.
func (e *CommitmentTrees) TangleRoot() []byte {
	return e.tangleTree.Root()
}

// StateMutationRoot returns the root of the state mutation sparse merkle tree.
func (e *CommitmentTrees) StateMutationRoot() []byte {
	return e.stateMutationTree.Root()
}

// EpochCommitmentFactory manages epoch commitmentTrees.
type EpochCommitmentFactory struct {
	commitmentTrees  map[epoch.EI]*CommitmentTrees
	commitmentsMutex sync.RWMutex

	ecc map[epoch.EI]*epoch.EC

	storage *EpochCommitmentStorage

	// FullEpochIndex is the epoch index we have the full ledger state for.
	// This index also represents the full ledger state dumped at snapshot.
	FullEpochIndex epoch.EI

	// DiffEpochIndex is the epoch index up to which we have a diff, starting from FullEpochIndex.
	DiffEpochIndex epoch.EI

	// LastCommittedEpoch is the last epoch that was committed, the in-memory
	LastCommittedEpoch epoch.EI

	// The state tree that always lags behind and gets the diffs applied to upon epoch commitment.
	stateRootTree *smt.SparseMerkleTree

	tangle     *tangle.Tangle
	hasher     hash.Hash
	ECMaxDepth uint64
}

// NewEpochCommitmentFactory returns a new commitment factory.
func NewEpochCommitmentFactory(store kvstore.KVStore, vm vm.VM, tangle *tangle.Tangle) *EpochCommitmentFactory {
	hasher, _ := blake2b.New256(nil)

	epochCommitmentStorage := newEpochCommitmentStorage(WithStore(store), WithVM(vm))

	stateRootTreeStore := specializeStore(epochCommitmentStorage.baseStore, PrefixTree)
	stateRootTreeNodeStore := specializeStore(stateRootTreeStore, PrefixTreeNodes)
	stateRootTreeValueStore := specializeStore(stateRootTreeStore, PrefixTreeValues)

	return &EpochCommitmentFactory{
		commitmentTrees: make(map[epoch.EI]*CommitmentTrees),
		storage:         epochCommitmentStorage,
		hasher:          hasher,
		tangle:          tangle,
		ECMaxDepth:      ECCreationMaxDepth, // TODO replace this with the snapshotting time parameter
		stateRootTree:   smt.NewSparseMerkleTree(stateRootTreeNodeStore, stateRootTreeValueStore, hasher),
	}
}

// StateRoot returns the root of the state sparse merkle tree.
func (f *EpochCommitmentFactory) StateRoot() []byte {
	return f.stateRootTree.Root()
}

// NewCommitment returns an empty commitment for the epoch.
func (f *EpochCommitmentFactory) newCommitmentTrees(ei epoch.EI, prevECR *epoch.MerkleRoot) *CommitmentTrees {
	// Volatile storage for small trees
	db, _ := database.NewMemDB()
	messageIDStore := db.NewStore()
	messageValueStore := db.NewStore()
	stateMutationIDStore := db.NewStore()
	stateMutationValueStore := db.NewStore()

	commitmentTrees := &CommitmentTrees{
		EI:                ei,
		tangleTree:        smt.NewSparseMerkleTree(messageIDStore, messageValueStore, f.hasher),
		stateMutationTree: smt.NewSparseMerkleTree(stateMutationIDStore, stateMutationValueStore, f.hasher),
		prevECR:           prevECR,
	}

	return commitmentTrees
}

// ECR generates the epoch commitment root.
func (f *EpochCommitmentFactory) ECR(ei epoch.EI) (*epoch.ECR, error) {
	commitment, err := f.GetCommitment(ei)
	if err != nil {
		return nil, errors.Wrap(err, "ECR could not be created")
	}
	branch1 := types.NewIdentifier(append(commitment.prevECR.Bytes(), commitment.tangleRoot.Bytes()...))
	branch2 := types.NewIdentifier(append(commitment.stateRoot.Bytes(), commitment.stateMutationRoot.Bytes()...))
	var root []byte
	root = append(root, branch1.Bytes()...)
	root = append(root, branch2.Bytes()...)
	return &epoch.ECR{types.NewIdentifier(root)}, nil
}

// ECHash calculates the EC if not already stored.
func (f *EpochCommitmentFactory) ECHash(ei epoch.EI, depth uint64) (*epoch.EC, error) {
	if depth == 0 {
		return nil, errors.New("could not create EC, max depth achieved")
	}
	if ec, ok := f.ecc[ei]; ok {
		return ec, nil
	}
	ecr, err := f.ECR(ei)
	if err != nil {
		return nil, err
	}
	prevEC, err := f.ECHash(ei-1, depth-1)
	if err != nil {
		return nil, err
	}

	concatenated := append(prevEC.Bytes(), ecr.Bytes()...)
	concatenated = append(concatenated, byte(ei))
	EC := &epoch.EC{types.NewIdentifier(concatenated)}
	f.ecc[ei] = EC
	return EC, nil
}

// InsertTangleLeaf inserts msg to the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) InsertTangleLeaf(ei epoch.EI, msgID tangle.MessageID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting tangle leaf")
	}
	_, err = commitment.tangleTree.Update(msgID.Bytes(), msgID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the tangle tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while inserting tangle leaf")
	}
	return nil
}

// InsertStateLeaf inserts the outputID to the state sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateLeaf(ei epoch.EI, outputID utxo.OutputID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state leaf")
	}
	_, err = f.stateRootTree.Update(outputID.Bytes(), outputID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while inserting state leaf")
	}
	return nil
}

// InsertStateMutationLeaf inserts the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) InsertStateMutationLeaf(ei epoch.EI, txID utxo.TransactionID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while inserting state mutation leaf")
	}
	_, err = commitment.stateMutationTree.Update(txID.Bytes(), txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not insert leaf to the state mutation tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while inserting state mutation leaf")
	}
	return nil
}

// RemoveStateMutationLeaf deletes the transaction ID to the state mutation sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateMutationLeaf(ei epoch.EI, txID utxo.TransactionID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting state mutation leaf")
	}
	_, err = commitment.stateMutationTree.Delete(txID.Bytes())
	if err != nil {
		return errors.Wrap(err, "could not delete leaf from the state mutation tree")
	}
	err = f.updatePrevECR(commitment.EI)
	if err != nil {
		return errors.Wrap(err, "could not update prevECR while deleting state mutation leaf")
	}
	return nil
}

// RemoveTangleLeaf removes the message ID from the Tangle sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveTangleLeaf(ei epoch.EI, msgID tangle.MessageID) error {
	commitment, err := f.getOrCreateCommitment(ei)
	if err != nil {
		return errors.Wrap(err, "could not get commitment while deleting tangle leaf")
	}
	exists, _ := commitment.tangleTree.Has(msgID.Bytes())
	if exists {
		_, err2 := commitment.tangleTree.Delete(msgID.Bytes())
		if err2 != nil {
			return errors.Wrap(err, "could not delete leaf from the tangle tree")
		}
		err2 = f.updatePrevECR(commitment.EI)
		if err2 != nil {
			return errors.Wrap(err, "could not update prevECR while deleting tangle leaf")
		}
	}
	return nil
}

// RemoveStateLeaf removes the output ID from the ledger sparse merkle tree.
func (f *EpochCommitmentFactory) RemoveStateLeaf(ei epoch.EI, outID utxo.OutputID) error {
	exists, _ := f.stateRootTree.Has(outID.Bytes())
	if exists {
		_, err := f.stateRootTree.Delete(outID.Bytes())
		if err != nil {
			return errors.Wrap(err, "could not delete leaf from the state tree")
		}
		err = f.updatePrevECR(ei)
		if err != nil {
			return errors.Wrap(err, "could not update prevECR while deleting state leaf")
		}
	}
	return nil
}

// GetCommitment returns the commitment with the given ei.
func (f *EpochCommitmentFactory) GetCommitment(ei epoch.EI) (*Commitment, error) {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()
	commitmentTrees := f.commitmentTrees[ei]
	stateRoot, err := f.getStateRoot(ei)
	if err != nil {
		return nil, err
	}
	commitment := &Commitment{}
	commitment.EI = ei
	// convert []byte to [32]byte type
	copy(commitment.stateRoot.Bytes(), commitmentTrees.tangleTree.Root())
	copy(commitment.stateMutationRoot.Bytes(), commitmentTrees.stateMutationTree.Root())
	copy(commitment.stateRoot.Bytes(), stateRoot)
	commitment.prevECR = commitmentTrees.prevECR

	return commitment, nil
}

// GetEpochCommitment returns the epoch commitment with the given ei.
func (f *EpochCommitmentFactory) GetEpochCommitment(ei epoch.EI) (*epoch.EpochCommitment, error) {
	ecr, err := f.ECR(ei)
	if err != nil {
		return nil, errors.Wrapf(err, "epoch commitment could not be created for epoch %d", ei)
	}
	prevECR, err := f.ECHash(ei-1, f.ECMaxDepth)
	if err != nil {
		return nil, errors.Wrapf(err, "epoch commitment could not be created for epoch %d", ei)
	}
	return &epoch.EpochCommitment{
		EI:         uint64(ei),
		ECR:        ecr,
		PreviousEC: prevECR,
	}, nil
}

func (f *EpochCommitmentFactory) getOrCreateCommitment(ei epoch.EI) (commitmentTrees *CommitmentTrees, err error) {
	f.commitmentsMutex.RLock()
	commitmentTrees, ok := f.commitmentTrees[ei]
	f.commitmentsMutex.RUnlock()
	if !ok {
		var previousECR *epoch.ECR

		if ei > 0 {
			previousECR, err = f.ECR(ei - 1)
			if err != nil {
				return nil, err
			}
		}
		commitmentTrees = f.newCommitmentTrees(ei, previousECR)
		f.commitmentsMutex.Lock()
		f.commitmentTrees[ei] = commitmentTrees
		f.commitmentsMutex.Unlock()
	}
	return
}

// ProofStateRoot returns the merkle proof for the outputID against the state root.
func (f *EpochCommitmentFactory) ProofStateRoot(ei epoch.EI, outID utxo.OutputID) (*CommitmentProof, error) {
	key := outID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.stateRootTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofStateMutationRoot returns the merkle proof for the transactionID against the state mutation root.
func (f *EpochCommitmentFactory) ProofStateMutationRoot(ei epoch.EI, txID utxo.TransactionID) (*CommitmentProof, error) {
	key := txID.Bytes()
	root := f.commitmentTrees[ei].stateMutationTree.Root()
	proof, err := f.commitmentTrees[ei].stateMutationTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the state mutation root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// ProofTangleRoot returns the merkle proof for the blockID against the tangle root.
func (f *EpochCommitmentFactory) ProofTangleRoot(ei epoch.EI, blockID tangle.MessageID) (*CommitmentProof, error) {
	key := blockID.Bytes()
	root := f.commitmentTrees[ei].tangleTree.Root()
	proof, err := f.commitmentTrees[ei].tangleTree.ProveForRoot(key, root)
	if err != nil {
		return nil, errors.Wrap(err, "could not generate the tangle root proof")
	}
	return &CommitmentProof{ei, proof, root}, nil
}

// VerifyTangleRoot verify the provided merkle proof against the tangle root.
func (f *EpochCommitmentFactory) VerifyTangleRoot(proof CommitmentProof, blockID tangle.MessageID) bool {
	key := blockID.Bytes()
	return f.verifyRoot(proof, key, key)
}

// VerifyStateMutationRoot verify the provided merkle proof against the state mutation root.
func (f *EpochCommitmentFactory) VerifyStateMutationRoot(proof CommitmentProof, transactionID utxo.TransactionID) bool {
	key := transactionID.Bytes()
	return f.verifyRoot(proof, key, key)
}

func (f *EpochCommitmentFactory) verifyRoot(proof CommitmentProof, key []byte, value []byte) bool {
	return smt.VerifyProof(proof.proof, proof.root, key, value, f.hasher)
}

func (f *EpochCommitmentFactory) updatePrevECR(prevEI epoch.EI) error {
	f.commitmentsMutex.RLock()
	defer f.commitmentsMutex.RUnlock()

	forwardCommitment, ok := f.commitmentTrees[prevEI+1]
	if !ok {
		return nil
	}
	prevECR, err := f.ECR(prevEI)
	if err != nil {
		return errors.Wrap(err, "could not update previous ECR")
	}
	forwardCommitment.prevECR = prevECR
	return nil
}

func (f *EpochCommitmentFactory) getStateRoot(ei epoch.EI) ([]byte, error) {
	if ei != f.LastCommittedEpoch+1 {
		return []byte{}, errors.Errorf("getting the state root of not next committable epoch is not supported")
	}
	return f.stateRootTree.Root(), nil
}

func (f *EpochCommitmentFactory) storeDiffUTXOs(ei epoch.EI, spent utxo.OutputIDs, created devnetvm.Outputs) {
	f.storage.diffsStore.Load(ei.Bytes()).Consume(func(epochDiff *epoch.EpochDiff) {
		for _, o := range created {
			epochDiff.M.Created.Add(o)
		}
		for it := spent.Iterator(); it.HasNext(); {
			out := f.tangle.Ledger.Storage.CachedOutput(it.Next())
			out.Consume(func(out utxo.Output) {
				outVM := out.(devnetvm.Output)
				epochDiff.M.Spent.Add(outVM)
			})
		}
		epochDiff.SetModified()
		epochDiff.Persist()
	})
}

type CommitmentProof struct {
	EI    epoch.EI
	proof smt.SparseMerkleProof
	root  []byte
}
