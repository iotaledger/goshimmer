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
	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/logger"
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

func parseMaster(s string) (*discover.Peer, error) {
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
	identity, err := id.NewIdentity(pubKey)
	if err != nil {
		return nil, errors.Wrap(err, "parseMaster")
	}

	return discover.NewPeer(identity, parts[1]), nil
}

func main() {
	var (
		listenAddr = flag.String("addr", "127.0.0.1:14626", "listen address")
		masterNode = flag.String("master", "", "master node as 'pubKey@address' where pubKey is in Base64")

		err error
	)
	flag.Parse()

	logger := logger.NewLogger(defaultZLC, "debug")
	defer logger.Sync()

	addr, err := net.ResolveUDPAddr("udp", *listenAddr)
	if err != nil {
		log.Fatalf("ResolveUDPAddr: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()

	cfg := discover.Config{
		ID:  id.GeneratePrivate(),
		Log: logger.Named("discover"),
	}

	master, err := parseMaster(*masterNode)
	if err != nil {
		log.Printf("Ignoring master: %v\n", err)
	} else if master != nil {
		cfg.Bootnodes = []*discover.Peer{master}
	}

	// start the discovery on that connection
	disc, err := discover.Listen(transport.Conn(conn, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) }), cfg)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer disc.Close()

	id := base64.StdEncoding.EncodeToString(disc.LocalID().ID())
	fmt.Println("Discovery protocol started: ID=" + id + ", address=" + disc.LocalAddr())
	fmt.Println("Hit Ctrl+C to exit")

	// wait for Ctrl+c
	waitInterrupt()
}
