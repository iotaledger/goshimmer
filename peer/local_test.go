package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/salt"
)

func TestNewLocal(t *testing.T) {
	got := NewLocal()

	assert.NotEqual(t, nil, got.Private.PublicKey)
}

func TestLocalPrivateSalt(t *testing.T) {
	p := NewLocal()

	salt, _ := salt.NewSalt(time.Second * 10)
	p.SetPrivateSalt(salt)

	got := p.GetPrivateSalt()

	assert.Equal(t, salt, got, "Private salt")
}

func TestLocalPublicSalt(t *testing.T) {
	p := NewLocal()

	salt, _ := salt.NewSalt(time.Second * 10)
	p.SetPublicSalt(salt)

	got := p.GetPublicSalt()

	assert.Equal(t, salt, got, "Public salt")
}
