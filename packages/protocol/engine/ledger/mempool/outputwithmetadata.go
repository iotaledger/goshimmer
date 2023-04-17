package mempool

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/mockedvm"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
	"github.com/iotaledger/hive.go/stringify"
)

// OutputWithMetadata represents an Output with its associated metadata fields that are needed for slot management.
type OutputWithMetadata struct {
	model.Storable[utxo.OutputID, OutputWithMetadata, *OutputWithMetadata, outputWithMetadataModel] `serix:"0"`
}

type outputWithMetadataModel struct {
	Index                 slot.Index    `serix:"0"`
	SpentInSlot           slot.Index    `serix:"1"`
	OutputID              utxo.OutputID `serix:"2"`
	Output                utxo.Output   `serix:"3"`
	ConsensusManaPledgeID identity.ID   `serix:"4"`
	AccessManaPledgeID    identity.ID   `serix:"5"`
}

// String returns a human-readable version of the OutputWithMetadata.
func (o *OutputWithMetadata) String() string {
	structBuilder := stringify.NewStructBuilder("OutputWithMetadata")
	structBuilder.AddField(stringify.NewStructField("Index", o.Index()))
	structBuilder.AddField(stringify.NewStructField("SpentInSlot", o.SpentInSlot()))
	structBuilder.AddField(stringify.NewStructField("OutputID", o.ID()))
	structBuilder.AddField(stringify.NewStructField("Output", o.Output()))
	structBuilder.AddField(stringify.NewStructField("ConsensusPledgeID", o.ConsensusManaPledgeID()))
	structBuilder.AddField(stringify.NewStructField("AccessPledgeID", o.AccessManaPledgeID()))

	return structBuilder.String()
}

// NewOutputWithMetadata returns a new OutputWithMetadata object.
func NewOutputWithMetadata(index slot.Index, outputID utxo.OutputID, output utxo.Output, consensusManaPledgeID, accessManaPledgeID identity.ID) (o *OutputWithMetadata) {
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
func (o *OutputWithMetadata) Index() slot.Index {
	o.RLock()
	defer o.RUnlock()

	return o.M.Index
}

// SetIndex sets the index of the output.
func (o *OutputWithMetadata) SetIndex(index slot.Index) {
	o.Lock()
	defer o.Unlock()

	o.M.Index = index
}

func (o *OutputWithMetadata) SpentInSlot() slot.Index {
	o.RLock()
	defer o.RUnlock()

	return o.M.SpentInSlot
}

// SetSpentInSlot sets the index of the epoc the output was spent in.
func (o *OutputWithMetadata) SetSpentInSlot(index slot.Index) {
	o.Lock()
	defer o.Unlock()

	o.M.SpentInSlot = index
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

	switch output := o.Output().(type) {
	case devnetvm.Output:
		return output.Balances().Get(devnetvm.ColorIOTA)
	case *mockedvm.MockedOutput:
		return output.M.Balance, true
	default:
		panic(fmt.Sprintf("unknown output type '%s'", output))
	}
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
