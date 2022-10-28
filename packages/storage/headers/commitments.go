package headers

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
)

type Commitments struct {
	*storable.Slice[commitment.Commitment, *commitment.Commitment]
}

func NewCommitments(path string) (newCommitment *Commitments, err error) {
	commitmentsSlice, err := storable.NewSlice[commitment.Commitment](path)
	if err != nil {
		return nil, errors.Errorf("failed to create commitments file: %w", err)
	}

	return &Commitments{
		Slice: commitmentsSlice,
	}, nil
}

func (c *Commitments) Commitment(index epoch.Index) (commitment *commitment.Commitment, err error) {
	if commitment, err = c.Get(int(index)); err != nil {
		return nil, errors.Errorf("failed to get commitment for epoch %d: %w", index, err)
	}

	return commitment, nil
}

func (c *Commitments) SetCommitment(index epoch.Index, commitment *commitment.Commitment) (err error) {
	if err = c.Set(int(index), commitment); err != nil {
		return errors.Errorf("failed to store commitment for epoch %d: %w", index, err)
	}

	return nil
}
