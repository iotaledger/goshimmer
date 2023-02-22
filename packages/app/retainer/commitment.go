package retainer

import (
	"context"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
)

// region cachedCommitment ///////////////////////////////////////////////////////////////////////////////////////////////

type cachedCommitment struct {
	*commitmentDetailsModel

	sync.Mutex
}

func newCachedCommitment() *cachedCommitment {
	return &cachedCommitment{commitmentDetailsModel: &commitmentDetailsModel{
		ID:                   commitment.ID{},
		Commitment:           nil,
		AcceptedBlocks:       make(models.BlockIDs),
		AcceptedTransactions: utxo.NewTransactionIDs(),
		CreatedOutputs:       utxo.NewOutputIDs(),
		SpentOutputs:         utxo.NewOutputIDs(),
	}}
}

func (c *cachedCommitment) setCommitment(cm *commitment.Commitment, blocks models.BlockIDs,
	txIDs utxo.TransactionIDs, created utxo.OutputIDs, spent utxo.OutputIDs) {
	c.Lock()
	defer c.Unlock()

	c.Commitment = cm
	c.ID = cm.ID()
	c.AcceptedBlocks = blocks
	c.AcceptedTransactions = txIDs
	c.CreatedOutputs = created
	c.SpentOutputs = spent
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

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

func newCommitmentDetails(cc *cachedCommitment) (c *CommitmentDetails) {
	if cc == nil {
		c = model.NewStorable[commitment.ID, CommitmentDetails](&commitmentDetailsModel{})
		c.SetID(commitment.ID{})
		return
	}

	cc.Lock()
	defer cc.Unlock()

	c = model.NewStorable[commitment.ID, CommitmentDetails](&commitmentDetailsModel{})
	c.SetID(cc.ID)
	c.M.ID = cc.ID
	c.M.Commitment = cc.Commitment
	c.M.AcceptedBlocks = cc.AcceptedBlocks
	c.M.AcceptedTransactions = cc.AcceptedTransactions
	c.M.CreatedOutputs = cc.CreatedOutputs
	c.M.SpentOutputs = cc.SpentOutputs

	return
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
