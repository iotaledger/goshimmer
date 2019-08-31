package peer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ID is a unique identifier for each peer.
type ID [sha256.Size]byte

// Bytes returns the byte slice representation of the ID
func (id ID) Bytes() []byte {
	return id[:]
}

// String returns a shortened version of the ID as a hex encoded string.
func (id ID) String() string {
	return hex.EncodeToString(id[:8])
}

// ParseID parses a hex encoded ID.
func ParseID(s string) (ID, error) {
	var id ID
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil {
		return id, err
	}
	if len(b) != len(ID{}) {
		return id, fmt.Errorf("invalid length: need %d hex chars", hex.EncodedLen(len(ID{})))
	}
	copy(id[:], b)
	return id, nil
}

// ID computes the ID corresponding to the given public key.
func (k PublicKey) ID() ID {
	return sha256.Sum256(k)
}
