package ledger

import (
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

// OutputWithMetadata represents an Output with its associated metadata fields that are needed for epoch management.
type OutputWithMetadata struct {
	model.Storable[utxo.OutputID, OutputWithMetadata, *OutputWithMetadata, outputWithMetadataModel] `serix:"0"`
}

type outputWithMetadataModel struct {
	Index                 epoch.Index   `serix:"0"`
	SpentInEpoch          epoch.Index   `serix:"1"`
	OutputID              utxo.OutputID `serix:"2"`
	Output                utxo.Output   `serix:"3"`
	ConsensusManaPledgeID identity.ID   `serix:"4"`
	AccessManaPledgeID    identity.ID   `serix:"5"`
}

// String returns a human-readable version of the OutputWithMetadata.
func (o *OutputWithMetadata) String() string {
	structBuilder := stringify.NewStructBuilder("OutputWithMetadata")
	structBuilder.AddField(stringify.NewStructField("Index", o.Index()))
	structBuilder.AddField(stringify.NewStructField("SpentInEpoch", o.SpentInEpoch()))
	structBuilder.AddField(stringify.NewStructField("OutputID", o.ID()))
	structBuilder.AddField(stringify.NewStructField("Output", o.Output()))
	structBuilder.AddField(stringify.NewStructField("ConsensusPledgeID", o.ConsensusManaPledgeID()))
	structBuilder.AddField(stringify.NewStructField("AccessPledgeID", o.AccessManaPledgeID()))

	return structBuilder.String()
}

// NewOutputWithMetadata returns a new OutputWithMetadata object.
func NewOutputWithMetadata(index epoch.Index, outputID utxo.OutputID, output utxo.Output, consensusManaPledgeID, accessManaPledgeID identity.ID) (o *OutputWithMetadata) {
	o = model.NewStorable[utxo.OutputID, OutputWithMetadata](&outputWithMetadataModel{
		Index:                 index,
		OutputID:              outputID,
		Output:                output,
		ConsensusManaPledgeID: consensusManaPledgeID,
		AccessManaPledgeID:    accessManaPledgeID,
	}, false)
	o.SetID(outputID)
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
	if consumedBytes, err = o.Storable.FromBytes(data); err == nil {
		o.M.Output.SetID(o.M.OutputID)
		o.SetID(o.M.OutputID)
	}

	return
}

// Index returns the index of the output.
func (o *OutputWithMetadata) Index() epoch.Index {
	o.RLock()
	defer o.RUnlock()

	return o.M.Index
}

func (o *OutputWithMetadata) SpentInEpoch() epoch.Index {
	o.RLock()
	defer o.RUnlock()

	return o.M.SpentInEpoch
}

// SetSpentInEpoch sets the index of the epoc the output was spent in.
func (o *OutputWithMetadata) SetSpentInEpoch(index epoch.Index) {
	o.Lock()
	defer o.Unlock()

	o.M.SpentInEpoch = index
}

// Output returns the Output field.
func (o *OutputWithMetadata) Output() (output utxo.Output) {
	o.RLock()
	defer o.RUnlock()

	return o.M.Output
}

// IOTABalance returns the IOTA balance of the Output.
// TODO: don't make the ledger depend on devnetvm
func (o *OutputWithMetadata) IOTABalance() (balance uint64, exists bool) {
	o.RLock()
	defer o.RUnlock()

	devnetVMOutput, ok := o.Output().(devnetvm.Output)
	if !ok {
		return 0, false
	}

	return devnetVMOutput.Balances().Get(devnetvm.ColorIOTA)
}

// SetOutput sets the Output field.
func (o *OutputWithMetadata) SetOutput(output utxo.Output) {
	o.Lock()
	defer o.Unlock()

	o.M.Output = output
	o.SetModified()
}

// ConsensusManaPledgeID returns the consensus pledge id of the output.
func (o *OutputWithMetadata) ConsensusManaPledgeID() (consensusPledgeID identity.ID) {
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
func (o *OutputWithMetadata) AccessManaPledgeID() (consensusPledgeID identity.ID) {
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
