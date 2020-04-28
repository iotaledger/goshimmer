package tipselector

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func Test(t *testing.T) {
	// create tip selector
	tipSelector := New()

	// check if first tips point to genesis
	trunk1, branch1 := tipSelector.Tips()
	assert.Equal(t, message.EmptyId, trunk1)
	assert.Equal(t, message.EmptyId, branch1)

	// create a message and attach it
	localIdentity1 := identity.GenerateLocalIdentity()
	message1 := message.New(trunk1, branch1, localIdentity1, time.Now(), 0, payload.NewData([]byte("testmessage")))
	tipSelector.AddTip(message1)

	// check if the tip shows up in the tip count
	assert.Equal(t, 1, tipSelector.TipCount())

	// check if next tips point to our first message
	trunk2, branch2 := tipSelector.Tips()
	assert.Equal(t, message1.Id(), trunk2)
	assert.Equal(t, message1.Id(), branch2)

	// create a 2nd message and attach it
	localIdentity2 := identity.GenerateLocalIdentity()
	message2 := message.New(message.EmptyId, message.EmptyId, localIdentity2, time.Now(), 0, payload.NewData([]byte("testmessage")))
	tipSelector.AddTip(message2)

	// check if the tip shows up in the tip count
	assert.Equal(t, 2, tipSelector.TipCount())

	// attach a message to our two tips
	localIdentity3 := identity.GenerateLocalIdentity()
	trunk3, branch3 := tipSelector.Tips()
	message3 := message.New(trunk3, branch3, localIdentity3, time.Now(), 0, payload.NewData([]byte("testmessage")))
	tipSelector.AddTip(message3)

	// check if the tip shows replaces the current tips
	trunk4, branch4 := tipSelector.Tips()
	assert.Equal(t, 1, tipSelector.TipCount())
	assert.Equal(t, message3.Id(), trunk4)
	assert.Equal(t, message3.Id(), branch4)
}
