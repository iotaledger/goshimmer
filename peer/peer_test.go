package peer

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/id"
	pb "github.com/wollac/autopeering/peer/proto"
	"github.com/wollac/autopeering/salt"
)

func newTestPeer() *Peer {
	prv := id.GeneratePrivate()
	p := &Peer{}
	p.Identity, _ = id.NewIdentity(prv.PublicKey)
	p.IP = net.ParseIP("127.0.0.1")
	p.Services = NewServiceMap()
	p.Services["autopeering"] = &TypePort{
		Type: pb.ConnType_TCP,
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

	assert.Equal(t, p.IP, got.IP, "IP address")

	assert.Equal(t, p.Services, got.Services, "Service")

	assert.Equal(t, p.Salt.Bytes, got.Salt.Bytes, "Salt")
	assert.Equal(t, p.Salt.ExpirationTime.Unix(), got.Salt.ExpirationTime.Unix(), "SameSaltExpirationTime")
}
