package mana

import (
	"testing"

	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
)

func TestIDFromStr(t *testing.T) {
	_identity := identity.GenerateIdentity()
	ID, err := IDFromStr(base58.Encode(_identity.ID().Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, _identity.ID(), ID)
}

func TestIDFromPubKey(t *testing.T) {
	_identity := identity.GenerateIdentity()
	ID, err := IDFromPubKey(_identity.PublicKey().String())
	assert.NoError(t, err)
	assert.Equal(t, _identity.ID(), ID)
}
