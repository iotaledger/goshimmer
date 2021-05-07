package jsonmodels

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region Address //////////////////////////////////////////////////////////////////////////////////////////////////////

// Address represents the JSON model of a ledgerstate.Address.
type Address struct {
	Type   string `json:"type"`
	Base58 string `json:"base58"`
}

// NewAddress returns an Address from the given ledgerstate.Address.
func NewAddress(address ledgerstate.Address) *Address {
	return &Address{
		Type:   address.Type().String(),
		Base58: address.Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output represents the JSON model of a ledgerstate.Output.
type Output struct {
	OutputID *OutputID         `json:"outputID,omitempty"`
	Type     string            `json:"type"`
	Balances map[string]uint64 `json:"balances"`
	Address  string            `json:"address"`
}

// NewOutput returns an Output from the given ledgerstate.Output.
func NewOutput(output ledgerstate.Output) *Output {
	return &Output{
		OutputID: NewOutputID(output.ID()),
		Type:     output.Type().String(),
		Balances: func() (mappedBalances map[string]uint64) {
			mappedBalances = make(map[string]uint64)
			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				mappedBalances[color.String()] = balance

				return true
			})

			return
		}(),
		Address: output.Address().Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputID represents the JSON model of a ledgerstate.OutputID.
type OutputID struct {
	Base58        string `json:"base58"`
	TransactionID string `json:"transactionID"`
	OutputIndex   uint16 `json:"outputIndex"`
}

// NewOutputID returns an OutputID from the given ledgerstate.OutputID.
func NewOutputID(outputID ledgerstate.OutputID) *OutputID {
	if outputID == ledgerstate.EmptyOutputID {
		return nil
	}

	return &OutputID{
		Base58:        outputID.Base58(),
		TransactionID: outputID.TransactionID().Base58(),
		OutputIndex:   outputID.OutputIndex(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata represents the JSON model of the ledgerstate.OutputMetadata.
type OutputMetadata struct {
	OutputID           *OutputID `json:"outputID"`
	BranchID           string    `json:"branchID"`
	Solid              bool      `json:"solid"`
	SolidificationTime int64     `json:"solidificationTime"`
	ConsumerCount      int       `json:"consumerCount"`
	FirstConsumer      string    `json:"firstConsumer"`
	Finalized          bool      `json:"finalized"`
}

// NewOutputMetadata returns the OutputMetadata from the given ledgerstate.OutputMetadata.
func NewOutputMetadata(outputMetadata *ledgerstate.OutputMetadata) *OutputMetadata {
	return &OutputMetadata{
		OutputID:           NewOutputID(outputMetadata.ID()),
		BranchID:           outputMetadata.BranchID().Base58(),
		Solid:              outputMetadata.Solid(),
		SolidificationTime: outputMetadata.SolidificationTime().Unix(),
		ConsumerCount:      outputMetadata.ConsumerCount(),
		FirstConsumer:      outputMetadata.FirstConsumer().Base58(),
		Finalized:          outputMetadata.Finalized(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the JSON model of a ledgerstate.Consumer.
type Consumer struct {
	TransactionID string `json:"transactionID"`
	Valid         string `json:"valid"`
}

// NewConsumer returns a Consumer from the given ledgerstate.Consumer.
func NewConsumer(consumer *ledgerstate.Consumer) *Consumer {
	return &Consumer{
		TransactionID: consumer.TransactionID().Base58(),
		Valid:         consumer.Valid().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch represents the JSON model of a ledgerstate.Branch.
type Branch struct {
	ID                 string   `json:"id"`
	Type               string   `json:"type"`
	Parents            []string `json:"parents"`
	ConflictIDs        []string `json:"conflictIDs,omitempty"`
	Liked              bool     `json:"liked"`
	MonotonicallyLiked bool     `json:"monotonicallyLiked"`
	Finalized          bool     `json:"finalized"`
	InclusionState     string   `json:"inclusionState"`
}

// NewBranch returns a Branch from the given ledgerstate.Branch.
func NewBranch(branch ledgerstate.Branch) Branch {
	return Branch{
		ID:   branch.ID().Base58(),
		Type: branch.Type().String(),
		Parents: func() []string {
			parents := make([]string, 0)
			for id := range branch.Parents() {
				parents = append(parents, id.Base58())
			}

			return parents
		}(),
		ConflictIDs: func() []string {
			if branch.Type() != ledgerstate.ConflictBranchType {
				return make([]string, 0)
			}

			conflictIDs := make([]string, 0)
			for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
				conflictIDs = append(conflictIDs, conflictID.Base58())
			}

			return conflictIDs
		}(),
		Liked:              branch.Liked(),
		MonotonicallyLiked: branch.MonotonicallyLiked(),
		Finalized:          branch.Finalized(),
		InclusionState:     branch.InclusionState().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the JSON model of a ledgerstate.ChildBranch.
type ChildBranch struct {
	BranchID   string `json:"branchID"`
	BranchType string `json:"type"`
}

// NewChildBranch returns a ChildBranch from the given ledgerstate.ChildBranch.
func NewChildBranch(childBranch *ledgerstate.ChildBranch) *ChildBranch {
	return &ChildBranch{
		BranchID:   childBranch.ChildBranchID().Base58(),
		BranchType: childBranch.ChildBranchType().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents the JSON model of a ledgerstate.Conflict.
type Conflict struct {
	OutputID  *OutputID `json:"outputID"`
	BranchIDs []string  `json:"branchIDs"`
}

// NewConflict returns a Conflict from the given ledgerstate.ConflictID.
func NewConflict(conflictID ledgerstate.ConflictID, branchIDs []ledgerstate.BranchID) *Conflict {
	return &Conflict{
		OutputID: NewOutputID(conflictID.OutputID()),
		BranchIDs: func() (mappedBranchIDs []string) {
			mappedBranchIDs = make([]string, 0)
			for _, branchID := range branchIDs {
				mappedBranchIDs = append(mappedBranchIDs, branchID.Base58())
			}

			return
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Transaction //////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents the JSON model of a ledgerstate.Transaction.
type Transaction struct {
	Version           ledgerstate.TransactionEssenceVersion `json:"version"`
	Timestamp         int64                                 `json:"timestamp"`
	AccessPledgeID    string                                `json:"accessPledgeID"`
	ConsensusPledgeID string                                `json:"consensusPledgeID"`
	Inputs            []*Input                              `json:"inputs"`
	Outputs           []*Output                             `json:"outputs"`
	UnlockBlocks      []*UnlockBlock                        `json:"unlockBlocks"`
	DataPayload       []byte                                `json:"dataPayload"`
}

// NewTransaction returns a Transaction from the given ledgerstate.Transaction.
func NewTransaction(transaction *ledgerstate.Transaction) *Transaction {
	inputs := make([]*Input, len(transaction.Essence().Inputs()))
	for i, input := range transaction.Essence().Inputs() {
		inputs[i] = NewInput(input)
	}

	outputs := make([]*Output, len(transaction.Essence().Outputs()))
	for i, output := range transaction.Essence().Outputs() {
		outputs[i] = NewOutput(output)
	}

	unlockBlocks := make([]*UnlockBlock, len(transaction.UnlockBlocks()))
	for i, unlockBlock := range transaction.UnlockBlocks() {
		unlockBlocks[i] = NewUnlockBlock(unlockBlock)
	}

	dataPayload := make([]byte, 0)
	if transaction.Essence().Payload() != nil {
		dataPayload = transaction.Essence().Payload().Bytes()
	}

	return &Transaction{
		Version:           transaction.Essence().Version(),
		Timestamp:         transaction.Essence().Timestamp().Unix(),
		AccessPledgeID:    base58.Encode(transaction.Essence().AccessPledgeID().Bytes()),
		ConsensusPledgeID: base58.Encode(transaction.Essence().ConsensusPledgeID().Bytes()),
		Inputs:            inputs,
		Outputs:           outputs,
		UnlockBlocks:      unlockBlocks,
		DataPayload:       dataPayload,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Input ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Input represents the JSON model of a ledgerstate.Input.
type Input struct {
	Type               string    `json:"type"`
	ReferencedOutputID *OutputID `json:"referencedOutputID,omitempty"`
}

// NewInput returns an Input from the given ledgerstate.Input.
func NewInput(input ledgerstate.Input) *Input {
	if input.Type() == ledgerstate.UTXOInputType {
		return &Input{
			Type:               input.Type().String(),
			ReferencedOutputID: NewOutputID(input.(*ledgerstate.UTXOInput).ReferencedOutputID()),
		}
	}

	return &Input{
		Type:               "UnknownInputType",
		ReferencedOutputID: nil,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UnlockBlock //////////////////////////////////////////////////////////////////////////////////////////////////

// UnlockBlock represents the JSON model of a ledgerstate.UnlockBlock.
type UnlockBlock struct {
	Type            string                    `json:"type"`
	ReferencedIndex uint16                    `json:"referencedIndex,omitempty"`
	SignatureType   ledgerstate.SignatureType `json:"signatureType,omitempty"`
	PublicKey       string                    `json:"publicKey,omitempty"`
	Signature       string                    `json:"signature,omitempty"`
}

// NewUnlockBlock returns an UnlockBlock from the given ledgerstate.UnlockBlock.
func NewUnlockBlock(unlockBlock ledgerstate.UnlockBlock) *UnlockBlock {
	result := &UnlockBlock{
		Type: unlockBlock.Type().String(),
	}

	switch unlockBlock.Type() {
	case ledgerstate.SignatureUnlockBlockType:
		signature, _, _ := ledgerstate.SignatureFromBytes(unlockBlock.Bytes())
		result.SignatureType = signature.Type()
		switch signature.Type() {
		case ledgerstate.ED25519SignatureType:
			signature, _, _ := ledgerstate.ED25519SignatureFromBytes(signature.Bytes())
			result.PublicKey = signature.PublicKey.String()
			result.Signature = signature.Signature.String()

		case ledgerstate.BLSSignatureType:
			signature, _, _ := ledgerstate.BLSSignatureFromBytes(signature.Bytes())
			result.Signature = signature.Signature.String()
		}
	case ledgerstate.ReferenceUnlockBlockType:
		referenceUnlockBlock, _, _ := ledgerstate.ReferenceUnlockBlockFromBytes(unlockBlock.Bytes())
		result.ReferencedIndex = referenceUnlockBlock.ReferencedIndex()
	}

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata represents the JSON model of the ledgerstate.TransactionMetadata.
type TransactionMetadata struct {
	TransactionID      string `json:"transactionID"`
	BranchID           string `json:"branchID"`
	Solid              bool   `json:"solid"`
	SolidificationTime int64  `json:"solidificationTime"`
	Finalized          bool   `json:"finalized"`
	LazyBooked         bool   `json:"lazyBooked"`
}

// NewTransactionMetadata returns the TransactionMetadata from the given ledgerstate.TransactionMetadata.
func NewTransactionMetadata(transactionMetadata *ledgerstate.TransactionMetadata) *TransactionMetadata {
	return &TransactionMetadata{
		TransactionID:      transactionMetadata.ID().Base58(),
		BranchID:           transactionMetadata.BranchID().Base58(),
		Solid:              transactionMetadata.Solid(),
		SolidificationTime: transactionMetadata.SolidificationTime().Unix(),
		Finalized:          transactionMetadata.Finalized(),
		LazyBooked:         transactionMetadata.LazyBooked(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionConsensusMetadata /////////////////////////////////////////////////////////////////////////////////

// TransactionConsensusMetadata represents the JSON model of the transaction's consensus metadata.
type TransactionConsensusMetadata struct {
	TransactionID string `json:"transactionID"`
	Timestamp     int64  `json:"timestamp"`
	Liked         bool   `json:"liked"`
	LoK           string `json:"lok"`
	FCOBTime1     int64  `json:"fcobTime1"`
	FCOBTime2     int64  `json:"fcobTime2"`
}

// NewTransactionConsensusMetadata returns the TransactionConsensusMetadata from the given transaction ID.
func NewTransactionConsensusMetadata(transactionID ledgerstate.TransactionID, opinion *fcob.Opinion) *TransactionConsensusMetadata {
	return &TransactionConsensusMetadata{
		TransactionID: transactionID.Base58(),
		Timestamp:     opinion.Timestamp().Unix(),
		Liked:         opinion.Liked(),
		LoK:           opinion.LevelOfKnowledge().String(),
		FCOBTime1:     opinion.FCOBTime1().Unix(),
		FCOBTime2:     opinion.FCOBTime2().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
