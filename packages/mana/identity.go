package mana

import (
	"fmt"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

// IDFromStr decodes and returns an ID from base58.
func IDFromStr(idStr string) (ID identity.ID, err error) {
	ID = identity.ID{}
	if idStr == "" {
		return
	}
	bytes, err := base58.Decode(idStr)
	if err != nil {
		err = fmt.Errorf("could not decode ID: %s, from base58: %w", idStr, err)
		return
	}
	copy(ID[:], bytes)
	return
}

// IDFromPubKey returns the ID from the given public key.
func IDFromPubKey(pubKey string) (ID identity.ID, err error) {
	ID = identity.ID{}
	if pubKey == "" {
		return
	}
	bytes, err := base58.Decode(pubKey)
	if err != nil {
		err = fmt.Errorf("could not decode public key: %s, from base58: %w", pubKey, err)
		return
	}
	_identity, err := identity.Parse(marshalutil.New(bytes))
	if err != nil {
		err = fmt.Errorf("could not parse public key: %s, %w", pubKey, err)
		return
	}

	copy(ID[:], _identity.ID().Bytes())
	return
}
