package mana

import (
	"testing"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestIDFromPubKey(t *testing.T) {
	_identity := identity.GenerateIdentity()
	ID, err := IDFromStr(base58.Encode(_identity.ID().Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, _identity.ID(), ID)
}
