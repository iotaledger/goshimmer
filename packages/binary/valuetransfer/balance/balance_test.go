package balance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromBytes(t *testing.T) {
	balance := New(COLOR_IOTA, 1337)

	marshaledBalance := balance.Bytes()

	assert.Equal(t, Length, len(marshaledBalance))
}
