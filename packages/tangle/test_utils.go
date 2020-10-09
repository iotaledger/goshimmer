package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

func newTestNonceMessage(nonce uint64) *Message {
	return NewMessage(EmptyMessageID, EmptyMessageID, time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{})
}

func newTestDataMessage(payloadString string) *Message {
	return NewMessage(EmptyMessageID, EmptyMessageID, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, parent1, parent2 MessageID) *Message {
	return NewMessage(parent1, parent2, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}
