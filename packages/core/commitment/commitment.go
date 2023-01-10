package commitment

import (
	"unsafe"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

const Size = unsafe.Sizeof(commitment{})

type Commitment struct {
	model.Immutable[Commitment, *Commitment, commitment] `serix:"0"`
}

type commitment struct {
	Index            epoch.Index      `serix:"0"`
	PrevID           ID               `serix:"1"`
	RootsID          types.Identifier `serix:"2"`
	CumulativeWeight int64            `serix:"3"`
}

func New(index epoch.Index, prevID ID, rootsID types.Identifier, cumulativeWeight int64) (newCommitment *Commitment) {
	return model.NewImmutable[Commitment](&commitment{
		Index:            index,
		PrevID:           prevID,
		RootsID:          rootsID,
		CumulativeWeight: cumulativeWeight,
	})
}

func (c *Commitment) ID() (id ID) {
	idBytes := blake2b.Sum256(lo.PanicOnErr(c.Bytes()))

	return NewID(c.M.Index, idBytes[:])
}

func (c *Commitment) PrevID() (prevID ID) {
	return c.M.PrevID
}

func (c *Commitment) Index() (index epoch.Index) {
	return c.M.Index
}

func (c *Commitment) RootsID() (rootsID types.Identifier) {
	return c.M.RootsID
}

func (c *Commitment) CumulativeWeight() (cumulativeWeight int64) {
	return c.M.CumulativeWeight
}
