package mana

import (
	"context"
	"crypto/sha256"
	"math"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// PersistableEvent is a persistable event.
type PersistableEvent struct {
	objectstorage.StorableObjectFlags
	Type          byte                      `serix:"0"` // pledge or revoke
	ManaType      Type                      `serix:"1"` // access or consensus
	NodeID        identity.ID               `serix:"2"`
	Time          time.Time                 `serix:"3"`
	TransactionID ledgerstate.TransactionID `serix:"4"`
	Amount        float64                   `serix:"5"`
	InputID       ledgerstate.OutputID      `serix:"6"` // for revoke event
	bytes         []byte
}

// ToStringKeys returns the keys (properties) of the persistable event as a list of strings.
func (p *PersistableEvent) ToStringKeys() []string {
	return []string{"type", "nodeID", "fullNodeID", "amount", "time", "manaType", "transactionID", "inputID"}
}

// ToStringValues returns the persistableEvents values as a string array.
func (p *PersistableEvent) ToStringValues() []string {
	_type := strconv.Itoa(int(p.Type))
	_nodeID := p.NodeID.String()
	_fullNodeID := base58.Encode(p.NodeID[:])
	_amount := strconv.FormatFloat(p.Amount, 'g', -1, 64)
	_time := strconv.FormatInt(p.Time.Unix(), 10)
	_manaType := p.ManaType.String()
	_txID := p.TransactionID.Base58()
	_inputID := p.InputID.Base58()
	return []string{_type, _nodeID, _fullNodeID, _amount, _time, _manaType, _txID, _inputID}
}

// Bytes marshals the persistable event into a sequence of bytes.
func (p *PersistableEvent) BytesOld() []byte {
	if bytes := p.bytes; bytes != nil {
		return bytes
	}
	// create marshal helper
	marshalUtil := marshalutil.New()
	marshalUtil.WriteByte(p.Type)
	marshalUtil.WriteByte(byte(p.ManaType))
	marshalUtil.WriteBytes(p.NodeID.Bytes())
	marshalUtil.WriteTime(p.Time)
	marshalUtil.WriteBytes(p.TransactionID.Bytes())
	marshalUtil.WriteUint64(math.Float64bits(p.Amount))
	marshalUtil.WriteBytes(p.InputID.Bytes())
	p.bytes = marshalUtil.Bytes()
	return p.bytes
}

// Bytes marshals the persistable event into a sequence of bytes.
func (p *PersistableEvent) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageKey returns the key of the persistable mana.
func (p *PersistableEvent) ObjectStorageKey() []byte {
	return p.Bytes()
}

// ObjectStorageValue returns the bytes of the event.
func (p *PersistableEvent) ObjectStorageValue() []byte {
	return p.Bytes()
}

// parseEvent unmarshals a PersistableEvent using the given marshalUtil (for easier marshaling/unmarshaling).
func parseEvent(marshalUtil *marshalutil.MarshalUtil) (result *PersistableEvent, err error) {
	eventType, err := marshalUtil.ReadByte()
	if err != nil {
		return
	}
	manaType, err := marshalUtil.ReadByte()
	if err != nil {
		return
	}
	nodeIDBytes, err := marshalUtil.ReadBytes(sha256.Size)
	if err != nil {
		return
	}
	nodeID := identity.ID{}
	copy(nodeID[:], nodeIDBytes)

	eventTime, err := marshalUtil.ReadTime()
	if err != nil {
		return
	}
	txIDBytes, err := marshalUtil.ReadBytes(ledgerstate.TransactionIDLength)
	if err != nil {
		return
	}
	txID := ledgerstate.TransactionID{}
	copy(txID[:], txIDBytes)

	_amount, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	amount := math.Float64frombits(_amount)
	inputIDBytes, err := marshalUtil.ReadBytes(ledgerstate.OutputIDLength)
	if err != nil {
		return
	}
	inputID := ledgerstate.OutputID{}
	copy(inputID[:], inputIDBytes)
	consumedBytes := marshalUtil.ReadOffset()

	result = &PersistableEvent{
		Type:          eventType,
		NodeID:        nodeID,
		Amount:        amount,
		Time:          eventTime,
		ManaType:      Type(manaType),
		TransactionID: txID,
		InputID:       inputID,
	}
	result.bytes = make([]byte, consumedBytes)
	copy(result.bytes, marshalUtil.Bytes())
	return
}

// FromObjectStorage creates an PersistableEvent from sequences of key and bytes.
func (p *PersistableEvent) FromObjectStorage(_, value []byte) (objectstorage.StorableObject, error) {
	return p.FromBytes(value)
}

// FromBytes unmarshalls bytes into a persistable event.
func (p *PersistableEvent) FromBytesOld(data []byte) (result *PersistableEvent, err error) {
	return parseEvent(marshalutil.New(data))
}

// FromBytes unmarshalls bytes into a persistable event.
func (p *PersistableEvent) FromBytes(data []byte) (result *PersistableEvent, err error) {
	if result = p; result == nil {
		result = new(PersistableEvent)
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data, result, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse SigLockedColoredOutput: %w", err)
		return
	}
	return
}

var _ objectstorage.StorableObject = new(PersistableEvent)
