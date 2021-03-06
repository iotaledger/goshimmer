package ledgerstate

import (
	"bytes"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region Color ////////////////////////////////////////////////////////////////////////////////////////////////////////

// AliasIDLength represents the length of a AliasID (amount of bytes).
const AliasIDLength = ColorLength

// AliasID represents a marker that is associated to a token balance and that can give tokens a certain "meaning".
// AliasID is calculated the same way as Color: hash of the OutputID
type AliasID Color

var AliasNil = AliasID{}

// AliasIDFromBytes unmarshals a AliasID from a sequence of bytes.
func AliasIDFromBytes(data []byte) (AliasID, int, error) {
	ret, c, err := ColorFromBytes(data)

	return AliasID(ret), c, err
}

// AliasIDFromBase58EncodedString creates a AliasID from a base58 encoded string.
func AliasIDFromBase58EncodedString(base58String string) (aliasCode AliasID, err error) {
	parsedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded AliasID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if aliasCode, _, err = AliasIDFromBytes(parsedBytes); err != nil {
		err = xerrors.Errorf("failed to parse AliasID from bytes: %w", err)
		return
	}

	return
}

// AliasIDFromMarshalUtil unmarshals a AliasID using a MarshalUtil (for easier unmarshaling).
func AliasIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (aliasCode AliasID, err error) {
	aliasCodeBytes, err := marshalUtil.ReadBytes(ColorLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse AliasID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(aliasCode[:], aliasCodeBytes)

	return
}

// Bytes marshals the AliasID into a sequence of bytes.
// TODO pointer receiver?
func (c AliasID) Bytes() []byte {
	return c[:]
}

// Base58 returns a base58 encoded version of the Color.
func (c AliasID) Base58() string {
	return base58.Encode(c.Bytes())
}

// String creates a human readable string of the Color.
func (c AliasID) String() string {
	return c.Base58()
}

// Compare offers a comparator for AliasCodes which returns -1 if otherColor is bigger, 1 if it is smaller and 0 if they are
// the same.
func (c AliasID) Compare(other AliasID) int {
	return bytes.Compare(c[:], other[:])
}
