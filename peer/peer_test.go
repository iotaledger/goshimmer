package peer

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	p.PublicSalt, _ = salt.NewSalt(time.Second * 10)
	return p
}

func TestMarshalUnmarshal(t *testing.T) {
	p := newTestPeer()
	data, err := Marshal(p)
	require.Equal(t, nil, err, p)

	got := &Peer{}
	err = Unmarshal(data, got)
	require.Equal(t, nil, err, p)

	assert.Equal(t, p.Identity, got.Identity, "Identity")

	assert.Equal(t, p.IP, got.IP, "IP address")

	assert.Equal(t, p.Services, got.Services, "Service")

	assert.Equal(t, p.PublicSalt.Bytes, got.PublicSalt.Bytes, "Salt")
	assert.Equal(t, p.PublicSalt.ExpirationTime.Unix(), got.PublicSalt.ExpirationTime.Unix(), "SameSaltExpirationTime")
}
