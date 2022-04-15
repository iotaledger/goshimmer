package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerixPayload(t *testing.T) {
	obj := NewPayload("test1", "test2", "test3")

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	objRestored, _, err := FromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}
