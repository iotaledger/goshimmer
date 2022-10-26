package chainstorage

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/stringify"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

const (
	spentType byte = iota
	createdType
)

type DiffStorage struct {
	chainStorage *ChainStorage
}

func (d *DiffStorage) StoreSpent(outputWithMetadata *OutputWithMetadata) (err error) {
	store, err := d.SpentStorage(outputWithMetadata.Index())
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return d.store(store, outputWithMetadata)
}

func (d *DiffStorage) StoreCreated(outputWithMetadata *OutputWithMetadata) (err error) {
	store, err := d.CreatedStorage(outputWithMetadata.Index())
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return d.store(store, outputWithMetadata)
}

func (d *DiffStorage) GetSpent(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	store, err := d.SpentStorage(index)
	if err != nil {
		return nil, errors.Errorf("failed to extend realm for storage: %w", err)
	}
	return d.get(store, outputID)
}

func (d *DiffStorage) GetCreated(index epoch.Index, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	store, err := d.CreatedStorage(index)
	if err != nil {
		d.chainStorage.Events.Error.Trigger(errors.Errorf("failed to extend realm for storage: %w", err))
	}
	return d.get(store, outputID)
}

func (d *DiffStorage) DeleteSpent(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := d.SpentStorage(index)
	if err != nil {
		d.chainStorage.Events.Error.Trigger(errors.Errorf("failed to extend realm for storage: %w", err))
	}

	return d.delete(store, outputID)
}

func (d *DiffStorage) DeleteCreated(index epoch.Index, outputID utxo.OutputID) (err error) {
	store, err := d.CreatedStorage(index)
	if err != nil {
		d.chainStorage.Events.Error.Trigger(errors.Errorf("failed to extend realm for storage: %w", err))
	}

	return d.delete(store, outputID)
}

func (d *DiffStorage) DeleteSpentOutputs(index epoch.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = d.DeleteSpent(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (d *DiffStorage) DeleteCreatedOutputs(index epoch.Index, outputIDs utxo.OutputIDs) (err error) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		if err = d.DeleteCreated(index, it.Next()); err != nil {
			return
		}
	}

	return nil
}

func (d *DiffStorage) StreamSpent(index epoch.Index, callback func(*OutputWithMetadata)) (err error) {
	store, err := d.SpentStorage(index)
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	d.stream(store, callback)

	return
}

func (d *DiffStorage) StreamCreated(index epoch.Index, callback func(*OutputWithMetadata)) (err error) {
	store, err := d.CreatedStorage(index)
	if err != nil {
		return errors.Errorf("failed to extend realm for storage: %w", err)
	}

	d.stream(store, callback)

	return
}

func (d *DiffStorage) Storage(index epoch.Index) (storage kvstore.KVStore) {
	return d.chainStorage.bucketedStorage(index, LedgerDiffStorageType)
}

func (d *DiffStorage) SpentStorage(index epoch.Index) (storage kvstore.KVStore, err error) {
	return d.Storage(index).WithExtendedRealm([]byte{spentType})
}

func (d *DiffStorage) CreatedStorage(index epoch.Index) (storage kvstore.KVStore, err error) {
	return d.Storage(index).WithExtendedRealm([]byte{createdType})
}

func (d *DiffStorage) store(store kvstore.KVStore, outputWithMetadata *OutputWithMetadata) (err error) {
	outputWithMetadataBytes := lo.PanicOnErr(outputWithMetadata.Bytes())
	if err := store.Set(lo.PanicOnErr(outputWithMetadata.ID().Bytes()), outputWithMetadataBytes); err != nil {
		return errors.Errorf("failed to store output with metadata %s: %w", outputWithMetadata.ID(), err)
	}
	return
}

func (d *DiffStorage) get(store kvstore.KVStore, outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	outputWithMetadataBytes, err := store.Get(lo.PanicOnErr(outputID.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Errorf("failed to get block %s: %w", outputID, err)
	}

	outputWithMetadata = new(OutputWithMetadata)
	if _, err = outputWithMetadata.FromBytes(outputWithMetadataBytes); err != nil {
		return nil, errors.Errorf("failed to parse output with metadata %s: %w", outputID, err)
	}
	outputWithMetadata.SetID(outputID)

	return
}

func (d *DiffStorage) stream(store kvstore.KVStore, callback func(*OutputWithMetadata)) {
	store.Iterate([]byte{}, func(idBytes kvstore.Key, outputWithMetadataBytes kvstore.Value) bool {
		outputID := new(utxo.OutputID)
		outputID.FromBytes(idBytes)
		outputWithMetadata := new(OutputWithMetadata)
		outputWithMetadata.FromBytes(outputWithMetadataBytes)
		outputWithMetadata.SetID(*outputID)
		callback(outputWithMetadata)
		return true
	})
}

func (d *DiffStorage) delete(store kvstore.KVStore, outputID utxo.OutputID) (err error) {
	if err := store.Delete(lo.PanicOnErr(outputID.Bytes())); err != nil {
		return errors.Errorf("failed to delete output %s: %w", outputID, err)
	}
	return
}

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
