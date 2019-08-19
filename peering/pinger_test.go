package peering

import (
	"crypto/sha256"
	"net"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/proto"
)

func TestSendPing(t *testing.T) {
	ch := make(chan *pb.Ping)
	sendPing := func(to *net.UDPAddr, ping *pb.Ping) error {
		ch <- ping
		return nil
	}

	pinger := NewPinger(time.Second, sendPing)
	pinger.Start()
	defer pinger.Stop()

	to := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8888,
	}

	if err := pinger.Ping(to); err != nil {
		t.Error(err)
	}

	select {
	case _ = <-ch:
	case <-time.After(time.Second):
		t.Error("No ping sent")
	}
}

func TestPingExpired(t *testing.T) {
	ch := make(chan *pb.Ping)
	sendPing := func(to *net.UDPAddr, ping *pb.Ping) error {
		ch <- ping
		return nil
	}

	pinger := NewPinger(time.Second, sendPing)
	pinger.Start()
	defer pinger.Stop()

	to := &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8888,
	}

	if err := pinger.Ping(to); err != nil {
		t.Error(err)
	}

	var ping *pb.Ping
	select {
	case ping = <-ch:
	case <-time.After(time.Second):
		t.Error("No ping sent")
	}

	data, _ := proto.Marshal(ping)
	hash := sha256.Sum256(data)
	pong := &pb.Pong{PingHash: hash[:]}

	// since it is not yet expired this should return the ping
	assertProto(t, pinger.PongReceived(pong), ping)

	// wait for the hash to expire
	time.Sleep(2 * time.Second)

	if pinger.PongReceived(pong) != nil {
		t.Error("Ping not expired")
	}
}
