package networkdelay

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

func TestSerixRequest(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	obj := NewPayload(ID(identity.New(keyPair.PublicKey).ID()), time.Now().UnixNano())

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := FromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}
