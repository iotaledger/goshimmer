package commitment

import (
	"github.com/iotaledger/hive.go/core/byteutils"
	"github.com/iotaledger/hive.go/core/generics/model"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

const Size = blake2b.Size256 + blake2b.Size256 + 8

type Commitment struct {
	model.Immutable[Commitment, *Commitment, commitment] `serix:"0"`
}

type commitment struct {
	PrevID  ID          `serix:"0"`
	Index   epoch.Index `serix:"1"`
	RootsID RootsID     `serix:"2"`
}

func New(prevID ID, index epoch.Index, rootsID RootsID) (newCommitment *Commitment) {
	return model.NewImmutable[Commitment](&commitment{
		PrevID:  prevID,
		Index:   index,
		RootsID: rootsID,
	})
}

func (c *Commitment) ID() (id ID) {
	return blake2b.Sum256(byteutils.ConcatBytes(c.M.PrevID.Bytes(), c.M.Index.Bytes(), c.M.RootsID.Bytes()))
}

func (c *Commitment) PrevID() (prevID ID) {
	return c.M.PrevID
}

func (c *Commitment) Index() (index epoch.Index) {
	return c.M.Index
}

func (c *Commitment) RootsID() (rootsID RootsID) {
	return c.M.RootsID
}
