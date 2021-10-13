package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayload(t *testing.T) {
	a := NewPayload("me", "you", "ciao")
	abytes := a.Bytes()
	b, _, err := FromBytes(abytes)
	require.NoError(t, err)
	assert.Equal(t, a, b)
}
