package identity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const TestMessage = "Hello World!"

func TestVerifySignature(t *testing.T) {
	msg := []byte(TestMessage)

	private := GeneratePrivateIdentity()
	sig := private.Sign(msg)

	valid := private.Identity.VerifySignature(msg, sig)
	assert.True(t, valid)
}
