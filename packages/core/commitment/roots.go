package commitment

import (
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/core/model"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
)

type Roots struct {
	model.Immutable[Roots, *Roots, roots] `serix:"0"`
}

type roots struct {
	TangleRoot        types.Identifier `serix:"0"`
	StateMutationRoot types.Identifier `serix:"1"`
	ActivityRoot      types.Identifier `serix:"4"`
	StateRoot         types.Identifier `serix:"2"`
	ManaRoot          types.Identifier `serix:"3"`
}

func NewRoots(tangleRoot, stateMutationRoot, activityRoot, stateRoot, manaRoot types.Identifier) (newRoots *Roots) {
	return model.NewImmutable[Roots](&roots{
		TangleRoot:        tangleRoot,
		StateMutationRoot: stateMutationRoot,
		ActivityRoot:      activityRoot,
		StateRoot:         stateRoot,
		ManaRoot:          manaRoot,
	})
}

func (r *Roots) ID() (id types.Identifier) {
	branch1Hashed := blake2b.Sum256(byteutils.ConcatBytes(r.M.TangleRoot.Bytes(), r.M.StateMutationRoot.Bytes()))
	branch2Hashed := blake2b.Sum256(byteutils.ConcatBytes(r.M.StateRoot.Bytes(), r.M.ManaRoot.Bytes()))
	rootHashed := blake2b.Sum256(byteutils.ConcatBytes(branch1Hashed[:], branch2Hashed[:]))

	return rootHashed
}

func (r *Roots) TangleRoot() (tangleRoot types.Identifier) {
	return r.M.TangleRoot
}

func (r *Roots) StateMutationRoot() (stateMutationRoot types.Identifier) {
	return r.M.StateMutationRoot
}

func (r *Roots) StateRoot() (stateRoot types.Identifier) {
	return r.M.StateRoot
}

func (r *Roots) ManaRoot() (manaRoot types.Identifier) {
	return r.M.ManaRoot
}

func (r *Roots) ActivityRoot() (activityRoot types.Identifier) {
	return r.M.ActivityRoot
}
