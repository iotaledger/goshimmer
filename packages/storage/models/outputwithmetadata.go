package models

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

// region OutputWithMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// OutputWithMetadata represents an Output with its associated metadata fields that are needed for epoch management.
type OutputWithMetadata struct {
	model.Storable[utxo.OutputID, OutputWithMetadata, *OutputWithMetadata, outputWithMetadataModel] `serix:"0"`
}

type outputWithMetadataModel struct {
	Index                 epoch.Index   `serix:"0"`
	OutputID              utxo.OutputID `serix:"1"`
	Output                utxo.Output   `serix:"2"`
	CreationTime          time.Time     `serix:"3"`
	ConsensusManaPledgeID identity.ID   `serix:"4"`
	AccessManaPledgeID    identity.ID   `serix:"5"`
}

// String returns a human-readable version of the OutputWithMetadata.
func (o *OutputWithMetadata) String() string {
	structBuilder := stringify.NewStructBuilder("OutputWithMetadata")
	structBuilder.AddField(stringify.NewStructField("OutputID", o.ID()))
	structBuilder.AddField(stringify.NewStructField("Output", o.Output()))
	structBuilder.AddField(stringify.NewStructField("CreationTime", o.CreationTime()))
	structBuilder.AddField(stringify.NewStructField("ConsensusPledgeID", o.ConsensusManaPledgeID()))
	structBuilder.AddField(stringify.NewStructField("AccessPledgeID", o.AccessManaPledgeID()))

	return structBuilder.String()
}

// NewOutputWithMetadata returns a new OutputWithMetadata object.
func NewOutputWithMetadata(index epoch.Index, outputID utxo.OutputID, output utxo.Output, creationTime time.Time, consensusManaPledgeID, accessManaPledgeID identity.ID) (new *OutputWithMetadata) {
	new = model.NewStorable[utxo.OutputID, OutputWithMetadata](&outputWithMetadataModel{
		Index:                 index,
		OutputID:              outputID,
		Output:                output,
		CreationTime:          creationTime,
		ConsensusManaPledgeID: consensusManaPledgeID,
		AccessManaPledgeID:    accessManaPledgeID,
	})
	new.SetID(outputID)
	return
}

// FromObjectStorage creates an OutputWithMetadata from sequences of key and bytes.
func (o *OutputWithMetadata) FromObjectStorage(key, value []byte) error {
	err := o.Storable.FromObjectStorage(key, value)
	o.M.Output.SetID(o.M.OutputID)
	o.SetID(o.M.OutputID)

	return err
}

// FromBytes unmarshals an OutputWithMetadata from a sequence of bytes.
func (o *OutputWithMetadata) FromBytes(data []byte) (consumedBytes int, err error) {
	consumedBytes, err = o.Storable.FromBytes(data)
	o.M.Output.SetID(o.M.OutputID)
	o.SetID(o.M.OutputID)

	return
}

// Index returns the index of the output.
func (o *OutputWithMetadata) Index() epoch.Index {
	o.RLock()
	defer o.RUnlock()

	return o.M.Index
}

// Output returns the Output field.
func (o *OutputWithMetadata) Output() (output utxo.Output) {
	o.RLock()
	defer o.RUnlock()

	return o.M.Output
}

// TODO: don't make the ledger depend on devnetvm
// IOTABalance returns the IOTA balance of the Output.
func (o *OutputWithMetadata) IOTABalance() (balance uint64, exists bool) {
	o.RLock()
	defer o.RUnlock()

	return o.Output().(devnetvm.Output).Balances().Get(devnetvm.ColorIOTA)
}

// SetOutput sets the Output field.
func (o *OutputWithMetadata) SetOutput(output utxo.Output) {
	o.Lock()
	defer o.Unlock()

	o.M.Output = output
	o.SetModified()
}

// CreationTime returns the CreationTime field.
func (o *OutputWithMetadata) CreationTime() (creationTime time.Time) {
	o.RLock()
	defer o.RUnlock()

	return o.M.CreationTime
}

// SetCreationTime sets the CreationTime field.
func (o *OutputWithMetadata) SetCreationTime(creationTime time.Time) {
	o.Lock()
	defer o.Unlock()

	o.M.CreationTime = creationTime
}

// ConsensusManaPledgeID returns the consensus pledge id of the output.
func (o *OutputWithMetadata) ConsensusManaPledgeID() (consensuPledgeID identity.ID) {
	o.RLock()
	defer o.RUnlock()

	return o.M.ConsensusManaPledgeID
}

// SetConsensusManaPledgeID sets the consensus pledge id of the output.
func (o *OutputWithMetadata) SetConsensusManaPledgeID(consensusPledgeID identity.ID) {
	o.Lock()
	defer o.Unlock()

	o.M.ConsensusManaPledgeID = consensusPledgeID
}

// AccessManaPledgeID returns the access pledge id of the output.
func (o *OutputWithMetadata) AccessManaPledgeID() (consensuPledgeID identity.ID) {
	o.RLock()
	defer o.RUnlock()

	return o.M.AccessManaPledgeID
}

// SetAccessManaPledgeID sets the access pledge id of the output.
func (o *OutputWithMetadata) SetAccessManaPledgeID(accessPledgeID identity.ID) {
	o.Lock()
	defer o.Unlock()

	o.M.AccessManaPledgeID = accessPledgeID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
