package manamodels

import (
	"context"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// PersistableEvent is a persistable event.
type PersistableEvent struct {
	objectstorage.StorableObjectFlags
	Type          byte               `serix:"0"` // pledge or revoke
	ManaType      Type               `serix:"1"` // access or consensus
	IssuerID      identity.ID        `serix:"2"`
	Time          time.Time          `serix:"3"`
	TransactionID utxo.TransactionID `serix:"4"`
	Amount        int64              `serix:"5"`
	InputID       utxo.OutputID      `serix:"6"` // for revoke event
	bytes         []byte
}

// ToStringKeys returns the keys (properties) of the persistable event as a list of strings.
func (p *PersistableEvent) ToStringKeys() []string {
	return []string{"type", "issuerID", "fullIssuerID", "amount", "time", "manaType", "transactionID", "inputID"}
}

// ToStringValues returns the persistableEvents values as a string array.
func (p *PersistableEvent) ToStringValues() []string {
	_type := strconv.Itoa(int(p.Type))
	_issuerID := p.IssuerID.String()
	_fullIssuerID := base58.Encode(p.IssuerID[:])
	_amount := strconv.FormatInt(p.Amount, 10)
	_time := strconv.FormatInt(p.Time.Unix(), 10)
	_manaType := p.ManaType.String()
	_txID := p.TransactionID.Base58()
	_inputID := p.InputID.Base58()
	return []string{_type, _issuerID, _fullIssuerID, _amount, _time, _manaType, _txID, _inputID}
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

// FromObjectStorage creates an PersistableEvent from sequences of key and bytes.
func (p *PersistableEvent) FromObjectStorage(_, value []byte) error {
	_, err := p.FromBytes(value)
	return err
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
