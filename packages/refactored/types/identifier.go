package types

import (
	"crypto/rand"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// region Identifier ///////////////////////////////////////////////////////////////////////////////////////////////////

// Identifier is the type that represents the identifier of a Transaction.
type Identifier [IdentifierLength]byte

// FromRandomness fills the Identifier with random information.
func (t *Identifier) FromRandomness() (err error) {
	_, err = rand.Read((*t)[:])
	return
}

// FromBytes unmarshals an Identifier from a sequence of bytes.
func (t *Identifier) FromBytes(bytes []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if err = t.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Identifier from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromBase58 creates an Identifier from a base58 encoded string.
func (t *Identifier) FromBase58(base58String string) (err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded Identifier (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if _, err = t.FromBytes(decodedBytes); err != nil {
		err = errors.Errorf("failed to parse Identifier from bytes: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an Identifier using a MarshalUtil (for easier unmarshalling).
func (t *Identifier) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	outputIdentifierBytes, err := marshalUtil.ReadBytes(IdentifierLength)
	if err != nil {
		err = errors.Errorf("failed to parse Identifier (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy((*t)[:], outputIdentifierBytes)

	return
}

func (t Identifier) RegisterAlias(alias string) {
	_identifierAliasesMutex.Lock()
	defer _identifierAliasesMutex.Unlock()

	_identifierAliases[t] = alias
}

func (t Identifier) Alias() (alias string) {
	_identifierAliasesMutex.RLock()
	defer _identifierAliasesMutex.RUnlock()

	if existingAlias, exists := _identifierAliases[t]; exists {
		return existingAlias
	}

	return t.Base58()
}

func (t Identifier) UnregisterAlias() {
	_identifierAliasesMutex.Lock()
	defer _identifierAliasesMutex.Unlock()

	delete(_identifierAliases, t)
}

// Bytes returns a marshaled version of the Identifier.
func (t Identifier) Bytes() []byte {
	return t[:]
}

// Base58 returns a base58 encoded version of the Identifier.
func (t Identifier) Base58() string {
	return base58.Encode(t[:])
}

// String creates a human-readable version of the Identifier.
func (t Identifier) String() string {
	return "Identifier(" + t.Alias() + ")"
}

// IdentifierLength contains the byte size of a marshaled Identifier.
const IdentifierLength = 32

var (
	_identifierAliases      = make(map[Identifier]string)
	_identifierAliasesMutex = sync.RWMutex{}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
