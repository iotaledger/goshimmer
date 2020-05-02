package balance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalUnmarshal(t *testing.T) {
	balance := New(ColorIOTA, 1337)
	assert.Equal(t, int64(1337), balance.Value())
	assert.Equal(t, ColorIOTA, balance.Color())

	marshaledBalance := balance.Bytes()
	assert.Equal(t, Length, len(marshaledBalance))

	restoredBalance, consumedBytes, err := FromBytes(marshaledBalance)
	if err != nil {
		panic(err)
	}
	assert.Equal(t, Length, consumedBytes)
	assert.Equal(t, balance.value, restoredBalance.Value())
	assert.Equal(t, balance.color, restoredBalance.Color())
}
