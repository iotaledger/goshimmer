package main

import (
	"flag"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/wollac/autopeering/grpc"
	"github.com/wollac/autopeering/identity"
	"github.com/wollac/autopeering/peering"
)

var port = flag.Int("port", 4444, "peering gRPC port")
var masterNodes = flag.String("master-nodes", "", "comma separated list of master nodes")

func main() {
	flag.Parse()

	prot := &grpc.Protocol{}
	prot.Start(fmt.Sprintf(":%d", *port))
	defer prot.Stop()

	nodes := strings.Split(*masterNodes, ",")
	addrs := make([]*net.UDPAddr, len(nodes))

	for _, node := range nodes {
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			panic("Invalid endpoint address: " + node)
		}
		addrs = append(addrs, addr)
	}

	id := identity.GeneratePrivateIdentity()
	peering := peering.NewPeering(id, prot.SendPing)
	prot.OnPing(peering.OnPing)
	prot.OnPong(peering.OnPong)

	// peering.Start(addrs)

	time.Sleep(5 * time.Second)
}
