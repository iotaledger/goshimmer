package tipselector

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func Test(t *testing.T) {
	// create tip selector
	tipSelector := New()

	// check if first tips point to genesis
	trunk1, branch1 := tipSelector.Tips()
	assert.Equal(t, message.EmptyID, trunk1)
	assert.Equal(t, message.EmptyID, branch1)

	// create a message and attach it
	message1 := newTestMessage(trunk1, branch1, "testmessage")
	tipSelector.AddTip(message1)

	// check if the tip shows up in the tip count
	assert.Equal(t, 1, tipSelector.TipCount())

	// check if next tips point to our first message
	trunk2, branch2 := tipSelector.Tips()
	assert.Equal(t, message1.ID(), trunk2)
	assert.Equal(t, message1.ID(), branch2)

	// create a 2nd message and attach it
	message2 := newTestMessage(message.EmptyID, message.EmptyID, "testmessage")
	tipSelector.AddTip(message2)

	// check if the tip shows up in the tip count
	assert.Equal(t, 2, tipSelector.TipCount())

	// attach a message to our two tips
	trunk3, branch3 := tipSelector.Tips()
	message3 := newTestMessage(trunk3, branch3, "testmessage")
	tipSelector.AddTip(message3)

	// check if the tip shows replaces the current tips
	trunk4, branch4 := tipSelector.Tips()
	assert.Equal(t, 1, tipSelector.TipCount())
	assert.Equal(t, message3.ID(), trunk4)
	assert.Equal(t, message3.ID(), branch4)
}

func newTestMessage(trunk, branch message.ID, payloadString string) *message.Message {
	return message.New(trunk, branch, time.Now(), ed25519.PublicKey{}, 0, payload.NewData([]byte(payloadString)), 0, ed25519.Signature{})
}
