package mana

import (
	"math/rand"
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
)

const testChoices = 10
const testIterations = 1000000

// TestRandChooser_PickObvious should picks the only non-zero weighted item.
func TestRandChooser_PickObvious(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	chooser := NewRandChooser(
		NewChoice(1, 0),
		NewChoice(2, 0),
		NewChoice(3, 50),
	)
	res := chooser.Pick(1)
	assert.Equal(t, res[0].(int), 3, "should have picked choice 3")
}

// TestRandChooser_Pick should pick high weighted items more frequently.
func TestRandChooser_Pick(t *testing.T) {
	counts := make(map[int]int)
	for i := 0; i < testIterations; i++ {
		rand.Seed(time.Now().UTC().UnixNano())

		choices := mockFrequencyChoices(testChoices)
		chooser := NewRandChooser(choices...)
		item := chooser.Pick(1)
		counts[item[0].(int)]++
	}

	// Ensure weight 0 results in no results
	if vzero := counts[0]; vzero != 0 {
		t.Error("0 weight should not have been picked: ", vzero, counts[0])
	}

	// Test that higher weighted items were chosen more often than lower weighted items.
	for i := 0; i < testChoices-1; i++ {
		j := i + 1
		if counts[i] > counts[j] {
			t.Error("value not lesser", i, counts[i], j, counts[j])
		}
	}
}

// mockFrequencyChoices returns a random list of unique randChoices.
func mockFrequencyChoices(n int) []RandChoice {
	var choices []RandChoice
	list := rand.Perm(n)
	for _, v := range list {
		c := NewChoice(v, v)
		choices = append(choices, c)
	}
	return choices
}
