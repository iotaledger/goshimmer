package identity

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

const TestMessage = "Hello World!"

func TestVerifySignature(t *testing.T) {
	msg := []byte(TestMessage)

	private := GeneratePrivateIdentity()
	sig := private.Sign(msg)

	valid := private.Identity.VerifySignature(msg, sig)

	assert.Equal(t, valid, true)
}
