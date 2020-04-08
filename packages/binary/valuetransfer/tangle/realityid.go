package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

type RealityId [transaction.IdLength]byte

// NewRealityId creates a new RealityId from a transaction Id.
func NewRealityId(transactionId transaction.Id) (realityId RealityId) {
	copy(realityId[:], transactionId.Bytes())

	return
}

func RealityIdFromBytes(bytes []byte) (result RealityId, err error, consumedBytes int) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	realityIdBytes, idErr := marshalUtil.ReadBytes(RealityIdLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], realityIdBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func RealityIdFromBase58(base58String string) (realityId RealityId, err error) {
	// decode string
	bytes, err := base58.Decode(base58String)
	if err != nil {
		return
	}

	// sanitize input
	if len(bytes) != RealityIdLength {
		err = fmt.Errorf("base58 encoded string does not match the length of a RealityId")

		return
	}

	// copy bytes to result
	copy(realityId[:], bytes)

	return
}

func ParseRealityId(marshalUtil *marshalutil.MarshalUtil) (result RealityId, err error) {
	var realityIdBytes []byte
	if realityIdBytes, err = marshalUtil.ReadBytes(RealityIdLength); err != nil {
		return
	}

	copy(result[:], realityIdBytes)

	return
}

// Bytes marshals the RealityId into a sequence of bytes.
func (realityId RealityId) Bytes() []byte {
	return realityId[:]
}

// String creates a base58 encoded version of the RealityId.
func (realityId RealityId) String() string {
	return base58.Encode(realityId[:])
}

// RealityIdLength encodes the length of a reality identifier - since realities get created by transactions, it has the
// same length as a transaction Id.
const RealityIdLength = transaction.IdLength
