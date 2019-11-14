package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/pkg/errors"
	"github.com/wollac/autopeering/discover"
	"github.com/wollac/autopeering/logger"
	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/selection"
	"github.com/wollac/autopeering/server"
	"github.com/wollac/autopeering/transport"
)

const defaultZLC = `{
	"level": "info",
	"development": false,
	"outputPaths": ["stdout"],
	"errorOutputPaths": ["stderr"],
	"encoding": "console",
	"encoderConfig": {
	  "timeKey": "ts",
	  "levelKey": "level",
	  "nameKey": "logger",
	  "callerKey": "caller",
	  "messageKey": "msg",
	  "stacktraceKey": "stacktrace",
	  "lineEnding": "",
	  "levelEncoder": "",
	  "timeEncoder": "iso8601",
	  "durationEncoder": "",
	  "callerEncoder": ""
	}
  }`

func waitInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func parseMaster(s string) (*peer.Peer, error) {
	if len(s) == 0 {
		return nil, nil
	}

	parts := strings.Split(s, "@")
	if len(parts) != 2 {
		return nil, errors.New("parseMaster")
	}
	pubKey, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.Wrap(err, "parseMaster")
	}

	return peer.NewPeer(pubKey, parts[1]), nil
}

func main() {
	var (
		listenAddr = flag.String("addr", "127.0.0.1:14626", "listen address")
		masterPeer = flag.String("master", "", "master node as 'pubKey@address' where pubKey is in Base64")

		err error
	)
	flag.Parse()

	logger := logger.NewLogger(defaultZLC, "debug")
	defer func() { _ = logger.Sync() }() // ignore the returned error

	addr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		log.Fatalf("ResolveUDPAddr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()

	var masterPeers []*peer.Peer
	master, err := parseMaster(*masterPeer)
	if err != nil {
		log.Printf("Ignoring master: %v\n", err)
	} else if master != nil {
		masterPeers = []*peer.Peer{master}
	}

	// use the UDP connection for transport
	trans := transport.Conn(conn, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) })
	defer trans.Close()

	// create a new local node
	db := peer.NewPersistentDB(logger.Named("db"))
	defer db.Close()
	local, err := peer.NewLocal(db)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}
	// add a service for the peering
	local.Services()["peering"] = peer.NetworkAddress{Network: "udp", Address: *listenAddr}

	discovery := discover.New(local, discover.Config{
		Log:         logger.Named("disc"),
		MasterPeers: masterPeers,
	})
	selection := selection.New(local, discovery, selection.Config{
		Log:          logger.Named("sel"),
		SaltLifetime: selection.DefaultSaltLifetime,
	})

	// start a server doing discovery and peering
	srv := server.Listen(local, trans, logger.Named("srv"), discovery, selection)
	defer srv.Close()

	// start the discovery on that connection
	discovery.Start(srv)
	defer discovery.Close()

	// start the peering on that connection
	selection.Start(srv)
	defer selection.Close()

	id := base64.StdEncoding.EncodeToString(local.PublicKey())
	fmt.Println("Discovery protocol started: ID=" + id + ", address=" + srv.LocalAddr())
	fmt.Println("Hit Ctrl+C to exit")

	// wait for Ctrl+c
	waitInterrupt()
}
