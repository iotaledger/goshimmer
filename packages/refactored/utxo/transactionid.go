package utxo

import (
	"crypto/rand"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

// TransactionIDLength contains the byte size of a TransactionID.
const TransactionIDLength = 32

// TransactionID is the type that represents the identifier of a Transaction.
type TransactionID [TransactionIDLength]byte

// EmptyTransactionID represents the identifier of the genesis Transaction.
var EmptyTransactionID TransactionID

// FromTransactionBytes sets the TransactionID from the given execution results.
func (t *TransactionID) FromTransactionBytes(transactionBytes []byte) {
	*t = blake2b.Sum256(transactionBytes)
}

// FromRandomness fills the TransactionID with random information.
func (t *TransactionID) FromRandomness() (err error) {
	_, err = rand.Read((*t)[:])

	return
}

// FromBytes unmarshals an TransactionID from a sequence of bytes.
func (t *TransactionID) FromBytes(bytes []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if err = t.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromBase58 creates an TransactionID from a base58 encoded string.
func (t *TransactionID) FromBase58(base58String string) (err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded TransactionID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if _, err = t.FromBytes(decodedBytes); err != nil {
		err = errors.Errorf("failed to parse TransactionID from bytes: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an TransactionID using a MarshalUtil (for easier unmarshalling).
func (t *TransactionID) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	outputIDBytes, err := marshalUtil.ReadBytes(TransactionIDLength)
	if err != nil {
		err = errors.Errorf("failed to parse TransactionID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy((*t)[:], outputIDBytes)

	return
}

// Bytes returns a marshaled version of the TransactionID.
func (t TransactionID) Bytes() []byte {
	return t[:]
}

// Base58 returns a base58 encoded version of the TransactionID.
func (t TransactionID) Base58() string {
	return base58.Encode(t[:])
}

// String creates a human-readable version of the TransactionID.
func (t TransactionID) String() string {
	return "TransactionID(" + t.Base58() + ")"
}
