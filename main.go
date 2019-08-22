package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/wollac/autopeering/discover"
	"github.com/wollac/autopeering/identity"
	"github.com/wollac/autopeering/transport"
	"go.uber.org/zap"
)

var port = flag.Int("port", 8080, "port of the server")

func init() {
	flag.Parse()
}

func initLogger() *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	return logger
}

func waitInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func main() {
	logger := initLogger()
	defer logger.Sync()

	// create a UDP connection
	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	defer conn.Close()

	cfg := discover.Config{
		ID:  identity.GeneratePrivateIdentity(),
		Log: logger,
	}

	// start the discovery on that connection
	disc, err := discover.Listen(transport.Conn(conn), cfg)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	defer disc.Close()

	fmt.Println("Discovery protocol started, listening on " + disc.LocalAddr())
	fmt.Println("Hit Ctrl+C to exit")

	// wait for Ctrl+c
	waitInterrupt()
}
