package retainer

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
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
	c = model.NewStorable[commitment.ID, CommitmentDetails](&commitmentDetailsModel{
		AcceptedBlocks:       make(models.BlockIDs),
		AcceptedTransactions: utxo.NewTransactionIDs(),
		SpentOutputs:         utxo.NewOutputIDs(),
		CreatedOutputs:       utxo.NewOutputIDs(),
	})
	c.SetID(commitment.ID{})

	return c
}

func (c *CommitmentDetails) setCommitment(e *notarization.SlotCommittedDetails) {
	c.M.Commitment = e.Commitment
	c.M.ID = e.Commitment.ID()
	c.SetID(e.Commitment.ID())

	_ = e.AcceptedBlocks.Stream(func(key models.BlockID) bool {
		c.M.AcceptedBlocks.Add(key)
		return true
	})
	_ = e.AcceptedTransactions.Stream(func(key utxo.TransactionID) bool {
		c.M.AcceptedTransactions.Add(key)
		return true
	})

	_ = e.SpentOutputs(func(owm *mempool.OutputWithMetadata) error {
		c.M.SpentOutputs.Add(owm.ID())
		return nil
	})

	_ = e.CreatedOutputs(func(owm *mempool.OutputWithMetadata) error {
		c.M.CreatedOutputs.Add(owm.ID())
		return nil
	})
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
