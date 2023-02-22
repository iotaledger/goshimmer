package retainer

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
)

type CommitmentDetails struct {
	model.Storable[commitment.ID, CommitmentDetails, *CommitmentDetails, commitmentDetailsModel] `serix:"0"`
}

type commitmentDetailsModel struct {
	ID commitment.ID `serix:"0"`

	Commitment           *commitment.Commitment `serix:"1"`
	AcceptedBlocks       models.BlockIDs        `serix:"2,lengthPrefixType=uint32"`
	AcceptedTransactions utxo.TransactionIDs    `serix:"3,lengthPrefixType=uint32"`
	CreatedOutputs       utxo.OutputIDs         `serix:"4,lengthPrefixType=uint32"`
	SpentOutputs         utxo.OutputIDs         `serix:"5,lengthPrefixType=uint32"`
}

func newCommitmentDetails() (c *CommitmentDetails) {
	c = model.NewStorable[commitment.ID, CommitmentDetails](&commitmentDetailsModel{})
	c.SetID(commitment.ID{})

	return c
}

func (c *CommitmentDetails) setCommitment(cm *commitment.Commitment, blocks models.BlockIDs,
	txIDs utxo.TransactionIDs, created utxo.OutputIDs, spent utxo.OutputIDs) {
	c.M.Commitment = cm
	c.M.ID = cm.ID()
	c.SetID(cm.ID())

	c.M.AcceptedBlocks = blocks
	c.M.AcceptedTransactions = txIDs
	c.M.CreatedOutputs = created
	c.M.SpentOutputs = spent
}

func (c *CommitmentDetails) Encode() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), c.M)
}

func (c *CommitmentDetails) Decode(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, &c.M)
}

func (c *CommitmentDetails) MarshalJSON() ([]byte, error) {
	return serix.DefaultAPI.JSONEncode(context.Background(), c.M)
}

func (c *CommitmentDetails) UnmarshalJSON(bytes []byte) error {
	return serix.DefaultAPI.JSONDecode(context.Background(), bytes, &c.M)
}
