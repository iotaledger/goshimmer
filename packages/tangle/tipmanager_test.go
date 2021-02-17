package tangle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTipManager(t *testing.T) {
	// create tip selector
	tipSelector := NewTipManager(New())

	// check if first tips point to genesis
	parents := tipSelector.Tips(2)
	// there has to be only one valid tip, the genesis message
	assert.Equal(t, 1, len(parents))
	assert.Equal(t, parents[0], EmptyMessageID)

	// create a message and attach it
	message1 := newTestParentsDataMessage("testmessage1", parents, []MessageID{})
	tipSelector.AddTip(message1)

	// check if the tip shows up in the tip count
	assert.Equal(t, 1, tipSelector.TipCount())

	// check if next tips point to our first message
	parents2 := tipSelector.Tips(2)
	assert.Equal(t, 1, len(parents2))
	assert.Contains(t, parents2, message1.ID())

	// create a 2nd message and attach it
	message2 := newTestParentsDataMessage("testmessage2", []MessageID{EmptyMessageID}, []MessageID{EmptyMessageID})
	tipSelector.AddTip(message2)

	// check if the tip shows up in the tip count
	assert.Equal(t, 2, tipSelector.TipCount())

	// attach a message to our two tips
	parents3 := tipSelector.Tips(2)
	message3 := newTestParentsDataMessage("testmessage3", parents3, []MessageID{})
	tipSelector.AddTip(message3)

	// check if the tip shows replaces the current tips
	parents4 := tipSelector.Tips(2)
	assert.Equal(t, 1, tipSelector.TipCount())
	assert.Equal(t, 1, len(parents4))
	assert.Contains(t, parents4, message3.ID())
	assert.Contains(t, parents4, message3.ID())

}
