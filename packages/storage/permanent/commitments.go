package permanent

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

type Commitments struct {
	slice *storable.Slice[commitment.Commitment, *commitment.Commitment]
}

func NewCommitments(path string) (newCommitment *Commitments) {
	commitmentsSlice, err := storable.NewSlice[commitment.Commitment](path)
	if err != nil {
		panic(errors.Errorf("failed to create commitments file: %w", err))
	}

	return &Commitments{
		slice: commitmentsSlice,
	}
}

func (c *Commitments) Store(index epoch.Index, commitment *commitment.Commitment) (err error) {
	if err = c.slice.Set(int(index), commitment); err != nil {
		return errors.Errorf("failed to store commitment for epoch %d: %w", index, err)
	}

	return nil
}

func (c *Commitments) Load(index epoch.Index) (commitment *commitment.Commitment, err error) {
	if commitment, err = c.slice.Get(int(index)); err != nil {
		return nil, errors.Errorf("failed to get commitment for epoch %d: %w", index, err)
	}

	return commitment, nil
}

func (c *Commitments) Close() (err error) {
	return c.slice.Close()
}
