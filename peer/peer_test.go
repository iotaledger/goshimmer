package peer

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/salt"
)

func newTestPeer() *Peer {
	prv := id.GeneratePrivate()
	p := &Peer{}
	p.Identity, _ = id.NewIdentity(prv.PublicKey)
	p.Address = net.ParseIP("127.0.0.1")
	p.Services = NewServiceMap()
	p.Services["autopeering"] = &TypePort{
		Type: TCP,
		Port: 8000,
	}
	p.Salt, _ = salt.NewSalt(time.Second * 10)
	return p
}

func TestMarshalUnmarshal(t *testing.T) {
	p := newTestPeer()
	data, err := Marshal(p)
	assert.Equal(t, nil, err, p)

	got := &Peer{}
	err = Unmarshal(data, got)
	assert.Equal(t, nil, err, p)

	assert.Equal(t, p.Identity, got.Identity, "Identity")

	assert.Equal(t, p.Address, got.Address, "Address")

	assert.Equal(t, p.Services, got.Services, "Service")

	assert.Equal(t, p.Salt.Bytes, got.Salt.Bytes, "Salt")
	assert.Equal(t, true, got.Salt.ExpirationTime.Equal(p.Salt.ExpirationTime), "SameSaltExpirationTime")
}
