package ledgerstate

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
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
	OutputID              utxo.OutputID `serix:"1"`
	Output                utxo.Output   `serix:"2"`
	ConsensusManaPledgeID identity.ID   `serix:"4"`
	AccessManaPledgeID    identity.ID   `serix:"5"`
}

// String returns a human-readable version of the OutputWithMetadata.
func (o *OutputWithMetadata) String() string {
	structBuilder := stringify.NewStructBuilder("OutputWithMetadata")
	structBuilder.AddField(stringify.NewStructField("OutputID", o.ID()))
	structBuilder.AddField(stringify.NewStructField("Output", o.Output()))
	structBuilder.AddField(stringify.NewStructField("ConsensusPledgeID", o.ConsensusManaPledgeID()))
	structBuilder.AddField(stringify.NewStructField("AccessPledgeID", o.AccessManaPledgeID()))

	return structBuilder.String()
}

// NewOutputWithMetadata returns a new OutputWithMetadata object.
func NewOutputWithMetadata(index epoch.Index, outputID utxo.OutputID, output utxo.Output, consensusManaPledgeID, accessManaPledgeID identity.ID) (new *OutputWithMetadata) {
	new = model.NewStorable[utxo.OutputID, OutputWithMetadata](&outputWithMetadataModel{
		Index:                 index,
		OutputID:              outputID,
		Output:                output,
		ConsensusManaPledgeID: consensusManaPledgeID,
		AccessManaPledgeID:    accessManaPledgeID,
	}, false)
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

func (o *OutputWithMetadata) Export(writer io.WriteSeeker) (err error) {
	o.RLock()
	defer o.RUnlock()

	if outputBytes, serializationErr := o.Bytes(); serializationErr != nil {
		return errors.Errorf("failed to marshal output: %w", serializationErr)
	} else if err = binary.Write(writer, binary.LittleEndian, uint64(len(outputBytes))); err != nil {
		return errors.Errorf("failed to write output size: %w", err)
	} else if err = binary.Write(writer, binary.LittleEndian, outputBytes); err != nil {
		return errors.Errorf("failed to write output: %w", err)
	}

	return nil
}

func (o *OutputWithMetadata) Import(reader io.ReadSeeker) (imported *OutputWithMetadata, err error) {
	o.Lock()
	defer o.Unlock()

	var outputSize uint64
	if err = binary.Read(reader, binary.LittleEndian, &outputSize); err != nil {
		return nil, errors.Errorf("failed to read output size: %w", err)
	} else if outputSize == 0 {
		return nil, nil
	}

	outputBytes := make([]byte, outputSize)
	if err = binary.Read(reader, binary.LittleEndian, outputBytes); err != nil {
		return nil, errors.Errorf("failed to read output: %w", err)
	} else if consumedBytes, parseErr := o.FromBytes(outputBytes); parseErr != nil {
		return nil, errors.Errorf("failed to unmarshal output: %w", parseErr)
	} else if consumedBytes != int(outputSize) {
		return nil, errors.Errorf("failed to unmarshal output: consumed bytes (%d) != expected bytes (%d)", consumedBytes, outputSize)
	}
	o.SetID(o.M.OutputID)

	return o, nil
}
