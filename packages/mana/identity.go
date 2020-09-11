package mana

import (
	"fmt"

	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
)

// IDFromPubKey returns the identity of a node from its public key in base58.
func IDFromPubKey(pubKey string) (ID identity.ID, err error) {
	ID = identity.ID{}
	if pubKey == "" {
		return
	}
	bytes, err := base58.Decode(pubKey)
	if err != nil {
		err = fmt.Errorf("could not parse public key: %s, as base58: %w", pubKey, err)
		return
	}
	copy(ID[:], bytes)
	return
}
