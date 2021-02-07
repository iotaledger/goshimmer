package tangle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeIssuanceSortedList(t *testing.T) {
	now := time.Now()
	list := timeIssuanceSortedList{
		&Message{issuingTime: now},
		&Message{issuingTime: now.Add(1 * time.Second)},
		&Message{issuingTime: now.Add(3 * time.Second)},
	}

	before := &Message{issuingTime: now.Add(-1 * time.Second)}
	list = list.insert(before)
	assert.Equal(t, before, list[0])

	after := &Message{issuingTime: now.Add(5 * time.Second)}
	list = list.insert(after)
	assert.Equal(t, after, list[len(list)-1])

	between := &Message{issuingTime: now.Add(2 * time.Second)}
	list = list.insert(between)
	assert.Equal(t, between, list[3])

}
