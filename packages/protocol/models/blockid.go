package models

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2/byteutils"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
	"github.com/iotaledger/hive.go/stringify"
)

// region BlockID ////////////////////////////////////////////////////////////////////////////////////////////////////

const BlockIDContextKey = "blockID"

// BlockID identifies a block via its BLAKE2b-256 hash of its bytes.
type BlockID struct {
	Identifier types.Identifier `serix:"0"`
	SlotIndex  slot.Index       `serix:"1"`
}

func (b BlockID) Index() slot.Index {
	return b.SlotIndex
}

// EmptyBlockID is an empty id.
var EmptyBlockID BlockID

// NewBlockID returns a new BlockID for the given data.
func NewBlockID(contentHash types.Identifier, signature ed25519.Signature, slotIndex slot.Index) BlockID {
	return BlockID{
		Identifier: blake2b.Sum256(byteutils.ConcatBytes(contentHash[:], signature[:])),
		SlotIndex:  slotIndex,
	}
}

func (b BlockID) EncodeJSON() (any, error) {
	return b.Base58(), nil
}

func (b *BlockID) DecodeJSON(val any) error {
	serialized, ok := val.(string)
	if !ok {
		return errors.New("incorrect type")
	}
	return b.FromBase58(serialized)
}

// FromBytes deserializes a BlockID from a byte slice.
func (b *BlockID) FromBytes(serialized []byte) (consumedBytes int, err error) {
	return serix.DefaultAPI.Decode(context.Background(), serialized, b, serix.WithValidation())
}

// FromBase58 un-serializes a BlockID from a base58 encoded string.
func (b *BlockID) FromBase58(base58EncodedString string) (err error) {
	s := strings.Split(base58EncodedString, ":")
	decodedBytes, err := base58.Decode(s[0])
	if err != nil {
		return errors.Wrap(err, "could not decode base58 encoded BlockID.Identifier")
	}
	slotIndex, err := strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return errors.Wrap(err, "could not decode BlockID.SlotIndex from string")
	}

	if _, err = serix.DefaultAPI.Decode(context.Background(), decodedBytes, &b.Identifier, serix.WithValidation()); err != nil {
		return errors.Wrap(err, "failed to decode BlockID")
	}
	b.SlotIndex = slot.Index(slotIndex)

	return nil
}

// FromRandomness generates a random BlockID.
func (b *BlockID) FromRandomness(optionalSlot ...slot.Index) (err error) {
	if err = b.Identifier.FromRandomness(); err != nil {
		return errors.Wrap(err, "could not create Identifier from randomness")
	}

	if len(optionalSlot) >= 1 {
		b.SlotIndex = optionalSlot[0]
	}

	return nil
}

// Alias returns the human-readable alias of the BlockID (or the base58 encoded bytes if no alias was set).
func (b BlockID) Alias() (alias string) {
	_BlockIDAliasesMutex.RLock()
	defer _BlockIDAliasesMutex.RUnlock()

	if existingAlias, exists := _BlockIDAliases[b]; exists {
		return existingAlias
	}

	return fmt.Sprintf("%s, %d", b.Identifier, int(b.SlotIndex))
}

// RegisterAlias allows to register a human-readable alias for the BlockID which will be used as a replacement for the
// String method.
func (b BlockID) RegisterAlias(alias string) {
	_BlockIDAliasesMutex.Lock()
	defer _BlockIDAliasesMutex.Unlock()

	_BlockIDAliases[b] = alias
}

// UnregisterAlias allows to unregister a previously registered alias.
func (b BlockID) UnregisterAlias() {
	_BlockIDAliasesMutex.Lock()
	defer _BlockIDAliasesMutex.Unlock()

	delete(_BlockIDAliases, b)
}

// Base58 returns a base58 encoded version of the BlockID.
func (b BlockID) Base58() (base58Encoded string) {
	return fmt.Sprintf("%s:%s", base58.Encode(b.Identifier[:]), strconv.FormatInt(int64(b.SlotIndex), 10))
}

// Length returns the byte length of a serialized BlockID.
func (b BlockID) Length() int {
	return BlockIDLength
}

// Bytes returns a serialized version of the BlockID.
func (b BlockID) Bytes() (serialized []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
}

// String returns a human-readable version of the BlockID.
func (b BlockID) String() (humanReadable string) {
	return "BlockID(" + b.Alias() + ")"
}

// CompareTo does a lexicographical comparison to another blockID.
// Returns 0 if equal, -1 if smaller, or 1 if larger than other.
// Passing nil as other will result in a panic.
func (b BlockID) CompareTo(other BlockID) int {
	return bytes.Compare(lo.PanicOnErr(b.Bytes()), lo.PanicOnErr(other.Bytes()))
}

// BlockIDFromContext returns the BlockID from the given context.
func BlockIDFromContext(ctx context.Context) BlockID {
	blockID, ok := ctx.Value(BlockIDContextKey).(BlockID)
	if !ok {
		return EmptyBlockID
	}
	return blockID
}

// BlockIDToContext adds the BlockID to the given context.
func BlockIDToContext(ctx context.Context, blockID BlockID) context.Context {
	//nolint:staticcheck // we are not expecting collisions due to using a string type for the key
	return context.WithValue(ctx, BlockIDContextKey, blockID)
}

func IsEmptyBlockID(blockID BlockID) bool {
	return blockID == EmptyBlockID
}

var (
	// _BlockIDAliases contains a dictionary of BlockIDs associated to their human-readable alias.
	_BlockIDAliases = make(map[BlockID]string)

	// _BlockIDAliasesMutex is the mutex that is used to synchronize access to the previous map.
	_BlockIDAliasesMutex = sync.RWMutex{}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockIDs ///////////////////////////////////////////////////////////////////////////////////////////////////

// BlockIDs is a set of BlockIDs where every BlockID is stored only once.
type BlockIDs map[BlockID]types.Empty

// NewBlockIDs construct a new BlockID collection from the optional BlockIDs.
func NewBlockIDs(blkIDs ...BlockID) BlockIDs {
	m := make(BlockIDs)
	for _, blkID := range blkIDs {
		m[blkID] = types.Void
	}

	return m
}

// FromBytes deserializes a BlockIDs from a byte slice.
func (m BlockIDs) FromBytes(serialized []byte) (consumedBytes int, err error) {
	var slice []BlockID
	if consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), serialized, &slice, serix.WithTypeSettings(serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32))); err != nil {
		return
	}
	for _, b := range slice {
		m.Add(b)
	}
	return
}

// Bytes returns a serialized version of the BlockIDs.
func (m BlockIDs) Bytes() (serialized []byte, err error) {
	return serix.DefaultAPI.Encode(context.Background(), m.Slice(), serix.WithTypeSettings(serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32)))
}

// Slice converts the set of BlockIDs into a slice of BlockIDs.
func (m BlockIDs) Slice() []BlockID {
	ids := make([]BlockID, 0)
	for key := range m {
		ids = append(ids, key)
	}
	return ids
}

// Clone creates a copy of the BlockIDs.
func (m BlockIDs) Clone() (clonedBlockIDs BlockIDs) {
	clonedBlockIDs = make(BlockIDs)
	for key, value := range m {
		clonedBlockIDs[key] = value
	}
	return
}

// Add adds a BlockID to the collection and returns the collection to enable chaining.
func (m BlockIDs) Add(blockID BlockID) BlockIDs {
	m[blockID] = types.Void

	return m
}

// AddAll adds all BlockIDs to the collection and returns the collection to enable chaining.
func (m BlockIDs) AddAll(blockIDs BlockIDs) BlockIDs {
	for blockID := range blockIDs {
		m.Add(blockID)
	}

	return m
}

// Remove removes a BlockID from the collection and returns the collection to enable chaining.
func (m BlockIDs) Remove(blockID BlockID) BlockIDs {
	delete(m, blockID)

	return m
}

// RemoveAll removes the BlockIDs from the collection and returns the collection to enable chaining.
func (m BlockIDs) RemoveAll(blockIDs BlockIDs) BlockIDs {
	for blockID := range blockIDs {
		m.Remove(blockID)
	}

	return m
}

// Empty checks if BlockIDs is empty.
func (m BlockIDs) Empty() (empty bool) {
	return len(m) == 0
}

// Contains checks if the given target BlockID is part of the BlockIDs.
func (m BlockIDs) Contains(target BlockID) (contains bool) {
	_, contains = m[target]
	return
}

// Subtract removes all other from the collection and returns the collection to enable chaining.
func (m BlockIDs) Subtract(other BlockIDs) BlockIDs {
	for blockID := range other {
		delete(m, blockID)
	}

	return m
}

// First returns the first element in BlockIDs (not ordered). This method only makes sense if there is exactly one
// element in the collection.
func (m BlockIDs) First() BlockID {
	for blockID := range m {
		return blockID
	}
	return EmptyBlockID
}

// Base58 returns a string slice of base58 BlockID.
func (m BlockIDs) Base58() (result []string) {
	result = make([]string, 0, len(m))
	for id := range m {
		result = append(result, id.Base58())
	}

	return
}

// String returns a human-readable Version of the BlockIDs.
func (m BlockIDs) String() string {
	if len(m) == 0 {
		return "BlockIDs{}"
	}

	result := "BlockIDs{\n"
	for blockID := range m {
		result += strings.Repeat(" ", stringify.IndentationSize) + blockID.String() + ",\n"
	}
	result += "}"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
