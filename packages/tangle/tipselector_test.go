package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
)

func TestMessageTipSelector(t *testing.T) {
	// create tip selector
	tipSelector := NewMessageTipSelector()

	// check if first tips point to genesis
	parent11, parent21 := tipSelector.Tips()
	assert.Equal(t, EmptyMessageID, parent11)
	assert.Equal(t, EmptyMessageID, parent21)

	// create a message and attach it
	message1 := newTipSelectorTestMessage(parent11, parent21, "testmessage1")
	tipSelector.AddTip(message1)

	// check if the tip shows up in the tip count
	assert.Equal(t, 1, tipSelector.TipCount())

	// check if next tips point to our first message
	parent12, parent22 := tipSelector.Tips()
	assert.Equal(t, message1.ID(), parent12)
	assert.Equal(t, message1.ID(), parent22)

	// create a 2nd message and attach it
	message2 := newTipSelectorTestMessage(EmptyMessageID, EmptyMessageID, "testmessage2")
	tipSelector.AddTip(message2)

	// check if the tip shows up in the tip count
	assert.Equal(t, 2, tipSelector.TipCount())

	// attach a message to our two tips
	parent13, parent23 := tipSelector.Tips()
	message3 := newTipSelectorTestMessage(parent13, parent23, "testmessage3")
	tipSelector.AddTip(message3)

	// check if the tip shows replaces the current tips
	parent14, parent24 := tipSelector.Tips()
	assert.Equal(t, 1, tipSelector.TipCount())
	assert.Equal(t, message3.ID(), parent14)
	assert.Equal(t, message3.ID(), parent24)
}

func newTipSelectorTestMessage(parent1, parent2 MessageID, payloadString string) *Message {
	return NewMessage(parent1, parent2, time.Now(), ed25519.PublicKey{}, 0, NewDataPayload([]byte(payloadString)), 0, ed25519.Signature{})
}
