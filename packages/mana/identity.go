package mana

import (
	"fmt"

	"github.com/iotaledger/hive.go/identity"
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
		err = fmt.Errorf("could not parse public key: %s, as base58: %w", idStr, err)
		return
	}
	copy(ID[:], bytes)
	return
}
