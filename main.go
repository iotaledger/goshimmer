package main

import (
	"flag"
	"log"
	"net"
	"strings"
)

var masterNodes = flag.String("master-nodes", "", "comma separated list of master nodes")

func main() {
	flag.Parse()

	nodes := strings.Split(*masterNodes, ",")
	addrs := make([]*net.UDPAddr, len(nodes))

	for _, node := range nodes {
		addr, err := net.ResolveUDPAddr("udp", node)
		if err != nil {
			panic("Invalid endpoint address: " + node)
		}
		addrs = append(addrs, addr)
	}

	log.Println(addrs)
}
