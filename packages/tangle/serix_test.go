package tangle

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/serix"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func TestSerix(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewGenericDataPayload([]byte{1, 1, 1, 1, 1})

	msg, err := NewMessage(NewParentMessageIDs().AddStrong(EmptyMessageID), time.Now(), keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.NoError(t, err)

	// fmt.Println(msg)

	s := serix.NewAPI()
	err = s.RegisterObjects((*payload.Payload)(nil), new(payload.GenericDataPayload))
	assert.NoError(t, err)

	result, err := s.Encode(context.Background(), msg)
	assert.NoError(t, err)

	fmt.Println("Bytes", len(msg.Bytes()), msg.Bytes())
	fmt.Println("Serix", len(result), result)

	assert.Equal(t, msg.Bytes(), result)
}
