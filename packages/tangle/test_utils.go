package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

func newTestNonceMessage(nonce uint64) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), nonce, ed25519.Signature{})
}

func newTestDataMessage(payloadString string) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, strongParents, weakParents []MessageID) *Message {
	return NewMessage(strongParents, weakParents, time.Now(), ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestAPOWMessage(t time.Time) *Message {
	return NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, t, ed25519.PublicKey{}, 0, payload.NewGenericDataPayload([]byte("test")), 0, ed25519.Signature{})
}
