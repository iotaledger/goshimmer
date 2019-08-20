package main

import (
	"flag"
	"fmt"
	"net"
	"strings"

	"github.com/wollac/autopeering/grpc"
)

var port = flag.Int("port", 4444, "peering gRPC port")
var masterNodes = flag.String("master-nodes", "", "comma separated list of master nodes")

func main() {
	flag.Parse()

	trans := grpc.Start(fmt.Sprintf(":%d", *port))
	defer trans.Close()

	nodes := strings.Split(*masterNodes, ",")
	addrs := make([]*net.UDPAddr, len(nodes))

	for _, node := range nodes {
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			panic("Invalid endpoint address: " + node)
		}
		addrs = append(addrs, addr)
	}
}
