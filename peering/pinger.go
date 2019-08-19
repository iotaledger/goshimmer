package peering

import (
	"crypto/sha256"
	"errors"
	"log"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/patrickmn/go-cache"
	pb "github.com/wollac/autopeering/proto"
)

const (
	// Interval after which a new ping can be issued
	pingInterval = 1 * time.Second
	// Size of the buffer to store ping targets
	pingBufferSize = 100
	// Version of the sent pings
	pingVersion = 0
)

type Pinger struct {
	// Cache of sent pings sorted by their hash
	cache *cache.Cache
	// Function to actually send a created ping
	sendPing func(to *net.UDPAddr, ping *pb.Ping) error
	// background service to create and send pings
	service *service
}

func NewPinger(pingTimeout time.Duration, sendPing func(to *net.UDPAddr, ping *pb.Ping) error) *Pinger {
	c := cache.New(pingTimeout, pingTimeout)
	p := &Pinger{
		cache:    c,
		sendPing: sendPing,
	}

	return p
}
func (p *Pinger) Ping(to *net.UDPAddr) error {
	select {
	case p.service.ping <- to:
	default:
		return errors.New("buffer full")
	}
	return nil
}

func (p *Pinger) PongReceived(pong *pb.Pong) *pb.Ping {

	key := string(pong.GetPingHash())
	val, exists := p.cache.Get(key)
	if !exists {
		return nil
	}

	// set the key to nil, as a delete would trigger the OnEvict
	if err := p.cache.Replace(key, nil, cache.DefaultExpiration); err != nil {
		// key has already expired
		return nil
	}

	// return the corresponding ping
	return val.(*pb.Ping)
}

type service struct {
	ping chan *net.UDPAddr
	stop chan bool
}

func (s *service) run(p *Pinger) {
	log.Println("pinger: service started")
	for {
		select {
		case to := <-s.ping:
			if err := p.preparePing(to); err != nil {
				log.Println("pinger: error creating ping packet", err)
			} else {
				time.Sleep(pingInterval)
			}
		case <-s.stop:
			log.Println("pinger: service stopped")
			return
		}
	}
}

func (p *Pinger) Start() {
	if p.service != nil {
		panic("pinger: service already running")
	}
	s := &service{
		ping: make(chan *net.UDPAddr, pingBufferSize),
		stop: make(chan bool),
	}
	p.service = s
	go s.run(p)
}

func (p *Pinger) Stop() {
	p.service.stop <- true
}

func (p *Pinger) preparePing(to *net.UDPAddr) error {

	ping := &pb.Ping{
		Version: pingVersion,
		To: &pb.RpcEndpoint{
			Ip:   to.IP.String(),
			Port: uint32(to.Port),
		},
	}

	data, err := proto.Marshal(ping)
	if err != nil {
		panic(err)
	}
	hash := sha256.Sum256(data)
	key := string(hash[:])

	p.cache.SetDefault(key, ping)

	return p.sendPing(to, ping)
}
