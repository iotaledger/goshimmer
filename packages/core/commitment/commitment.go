package commitment

import (
	"unsafe"

	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/core/model"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
)

const Size = unsafe.Sizeof(commitment{})

type Commitment struct {
	model.Immutable[Commitment, *Commitment, commitment] `serix:"0"`
}

type commitment struct {
	Index            slot.Index       `serix:"0"`
	PrevID           ID               `serix:"1"`
	RootsID          types.Identifier `serix:"2"`
	CumulativeWeight int64            `serix:"3"`
}

func New(index slot.Index, prevID ID, rootsID types.Identifier, cumulativeWeight int64) (newCommitment *Commitment) {
	return model.NewImmutable[Commitment](&commitment{
		Index:            index,
		PrevID:           prevID,
		RootsID:          rootsID,
		CumulativeWeight: cumulativeWeight,
	})
}

func NewEmptyCommitment() (newCommitment *Commitment) {
	return model.NewImmutable[Commitment](&commitment{})
}

func (c *Commitment) ID() (id ID) {
	idBytes := blake2b.Sum256(lo.PanicOnErr(c.Bytes()))

	return NewID(c.M.Index, idBytes[:])
}

func (c *Commitment) PrevID() (prevID ID) {
	return c.M.PrevID
}

func (c *Commitment) Index() (index slot.Index) {
	return c.M.Index
}

func (c *Commitment) RootsID() (rootsID types.Identifier) {
	return c.M.RootsID
}

func (c *Commitment) CumulativeWeight() (cumulativeWeight int64) {
	return c.M.CumulativeWeight
}

func (c *Commitment) Equals(other *Commitment) bool {
	return c.ID() == other.ID() && c.PrevID() == other.PrevID() && c.Index() == other.Index() &&
		c.RootsID() == other.RootsID() && c.CumulativeWeight() == other.CumulativeWeight()
}
