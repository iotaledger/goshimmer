package commitment

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

type ID struct {
	SlotIndex  slot.Index       `serix:"0"`
	Identifier types.Identifier `serix:"1"`
}

func NewID(index slot.Index, idBytes []byte) (newCommitmentID ID) {
	newCommitmentID.SlotIndex = index
	copy(newCommitmentID.Identifier[:], idBytes)

	return
}

func (b ID) Index() slot.Index {
	return b.SlotIndex
}

func (b ID) EncodeJSON() (any, error) {
	return b.Base58(), nil
}

func (b *ID) DecodeJSON(val any) error {
	serialized, ok := val.(string)
	if !ok {
		return errors.New("incorrect type")
	}
	return b.FromBase58(serialized)
}

// FromBytes deserializes a ID from a byte slice.
func (b *ID) FromBytes(serialized []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), serialized, b, serix.WithValidation())
}

// FromBase58 un-serializes a ID from a base58 encoded string.
func (b *ID) FromBase58(base58EncodedString string) (err error) {
	s := strings.Split(base58EncodedString, ":")
	decodedBytes, err := base58.Decode(s[0])
	if err != nil {
		return errors.Wrap(err, "could not decode base58 encoded ID.Identifier")
	}
	slotIndex, err := strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return errors.Wrap(err, "could not decode ID.SlotIndex from string")
	}

	if _, err = serix.DefaultAPI.Decode(context.Background(), decodedBytes, &b.Identifier, serix.WithValidation()); err != nil {
		return errors.Wrap(err, "failed to decode ID")
	}
	b.SlotIndex = slot.Index(slotIndex)

	return nil
}

// FromRandomness generates a random ID.
func (b *ID) FromRandomness(optionalSlot ...slot.Index) (err error) {
	if err = b.Identifier.FromRandomness(); err != nil {
		return errors.Wrap(err, "could not create Identifier from randomness")
	}

	if len(optionalSlot) >= 1 {
		b.SlotIndex = optionalSlot[0]
	}

	return nil
}

// Alias returns the human-readable alias of the ID (or the base58 encoded bytes if no alias was set).
func (b ID) Alias() (alias string) {
	_IDAliasesMutex.RLock()
	defer _IDAliasesMutex.RUnlock()

	if existingAlias, exists := _IDAliases[b]; exists {
		return existingAlias
	}

	return fmt.Sprintf("%d::%s", int(b.SlotIndex), b.Identifier.Base58())
}

// RegisterAlias allows to register a human-readable alias for the ID which will be used as a replacement for the
// String method.
func (b ID) RegisterAlias(alias string) {
	_IDAliasesMutex.Lock()
	defer _IDAliasesMutex.Unlock()

	_IDAliases[b] = alias
}

// UnregisterAlias allows to unregister a previously registered alias.
func (b ID) UnregisterAlias() {
	_IDAliasesMutex.Lock()
	defer _IDAliasesMutex.Unlock()

	delete(_IDAliases, b)
}

// Base58 returns a base58 encoded version of the ID.
func (b ID) Base58() (base58Encoded string) {
	return fmt.Sprintf("%s:%s", base58.Encode(b.Identifier[:]), strconv.FormatInt(int64(b.SlotIndex), 10))
}

// Length returns the byte length of a serialized ID.
func (b ID) Length() int {
	return types.IdentifierLength + b.SlotIndex.Length()
}

// Bytes returns a serialized version of the ID.
func (b ID) Bytes() (serialized []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
}

// String returns a human-readable version of the ID.
func (b ID) String() (humanReadable string) {
	return "commitment.ID(" + b.Alias() + ")"
}

// CompareTo does a lexicographical comparison to another blockID.
// Returns 0 if equal, -1 if smaller, or 1 if larger than other.
// Passing nil as other will result in a panic.
func (b ID) CompareTo(other ID) int {
	return bytes.Compare(lo.PanicOnErr(b.Bytes()), lo.PanicOnErr(other.Bytes()))
}

var (
	// _IDAliases contains a dictionary of BlockIDs associated to their human-readable alias.
	_IDAliases = make(map[ID]string)

	// _IDAliasesMutex is the mutex that is used to synchronize access to the previous map.
	_IDAliasesMutex = sync.RWMutex{}
)
