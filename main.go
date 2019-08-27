package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/wollac/autopeering/discover"
	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/transport"
	"go.uber.org/zap"
)

func waitInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func main() {
	var (
		listenAddr = flag.String("addr", ":14626", "listen address")

		err error
	)
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("NewLogger: %v", err)
	}
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
		Log: logger,
	}

	// start the discovery on that connection
	disc, err := discover.Listen(transport.Conn(conn), cfg)
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer disc.Close()

	fmt.Println("Discovery protocol started, listening on " + disc.LocalAddr())
	fmt.Println("Hit Ctrl+C to exit")

	// wait for Ctrl+c
	waitInterrupt()
}
