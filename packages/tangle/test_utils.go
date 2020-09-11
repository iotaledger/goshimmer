package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
)

func newTestNonceMessage(nonce uint64) *Message {
	return NewMessage(EmptyMessageID, EmptyMessageID, time.Time{}, ed25519.PublicKey{}, 0, NewDataPayload([]byte("test")), nonce, ed25519.Signature{})
}

func newTestDataMessage(payloadString string) *Message {
	return NewMessage(EmptyMessageID, EmptyMessageID, time.Now(), ed25519.PublicKey{}, 0, NewDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}

func newTestParentsDataMessage(payloadString string, parent1, parent2 MessageID) *Message {
	return NewMessage(parent1, parent2, time.Now(), ed25519.PublicKey{}, 0, NewDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}
