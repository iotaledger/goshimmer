package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_lowerCamelCase(t *testing.T) {
	assert.Equal(t, "idRegistry", lowerCamelCase("IdRegistry"))
	assert.Equal(t, "fpcListen", lowerCamelCase("FPCListen"))
	assert.Equal(t, "messageLayer", lowerCamelCase("MessageLayer"))
	assert.Equal(t, "fcob", lowerCamelCase("FCOB"))
	assert.Equal(t, "radio", lowerCamelCase("Radio"))
}
