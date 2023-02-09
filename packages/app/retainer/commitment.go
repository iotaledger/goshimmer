package retainer

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/pkg/errors"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(make(models.BlockIDs, 0), serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16))
	if err != nil {
		panic(errors.Wrap(err, "error registering BlockIDs type settings"))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(utxo.NewTransactionIDs(), serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16))
	if err != nil {
		panic(errors.Wrap(err, "error registering TransactionIDs type settings"))
	}
	err = serix.DefaultAPI.RegisterTypeSettings(utxo.NewOutputIDs(), serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint16))
	if err != nil {
		panic(errors.Wrap(err, "error registering OutputIDs type settings"))
	}
}

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
	AcceptedBlocks       models.BlockIDs        `serix:"2"`
	AcceptedTransactions utxo.TransactionIDs    `serix:"3"`
	CreatedOutputs       utxo.OutputIDs         `serix:"4"`
	SpentOutputs         utxo.OutputIDs         `serix:"5"`
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
