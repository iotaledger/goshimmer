package framework

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/iotaledger/goshimmer/plugins/webapi/getNeighbors"

	"github.com/iotaledger/goshimmer/client"
)

type Peer struct {
	name string
	ip   net.IP
	*client.GoShimmerAPI
	dockerCli *dockerclient.Client
	chosen    []getNeighbors.Neighbor
	accepted  []getNeighbors.Neighbor
}

func NewPeer(name string, ip net.IP, dockerCli *dockerclient.Client) *Peer {
	return &Peer{
		name:         name,
		ip:           ip,
		GoShimmerAPI: client.NewGoShimmerAPI(getWebApiBaseUrl(ip), http.Client{Timeout: 30 * time.Second}),
		dockerCli:    dockerCli,
	}
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer:{%s, %s, %s, %d}", p.name, p.ip.String(), p.BaseUrl(), p.TotalNeighbors())
}

func (p *Peer) TotalNeighbors() int {
	return len(p.chosen) + len(p.accepted)
}

func (p *Peer) SetNeighbors(chosen, accepted []getNeighbors.Neighbor) {
	p.chosen = make([]getNeighbors.Neighbor, len(chosen))
	copy(p.chosen, chosen)

	p.accepted = make([]getNeighbors.Neighbor, len(accepted))
	copy(p.accepted, accepted)
}

func (p *Peer) Logs() (io.ReadCloser, error) {
	return p.dockerCli.ContainerLogs(
		context.Background(),
		p.name,
		types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: false,
			Since:      "",
			Timestamps: false,
			Follow:     false,
			Tail:       "",
			Details:    false,
		})
}

func getAvailablePeers(dockerCli *dockerclient.Client) (peers []*Peer) {
	// get peer master
	if addr, err := net.LookupIP(hostnamePeerMaster); err != nil {
		fmt.Printf("Could not resolve %s\n", hostnamePeerMaster)
	} else {
		p := NewPeer(hostnamePeerMaster, addr[0], dockerCli)
		peers = append(peers, p)
	}

	// get peer replicas
	for i := 1; ; i++ {
		peerName := fmt.Sprintf("%s%d", hostnamePeerReplicaPrefix, i)
		if addr, err := net.LookupIP(peerName); err != nil {
			//fmt.Printf("Could not resolve %s\n", peerName)
			break
		} else {
			p := NewPeer(peerName, addr[0], dockerCli)
			peers = append(peers, p)
		}
	}
	return
}

func getWebApiBaseUrl(ip net.IP) string {
	return fmt.Sprintf("http://%s:%s", ip.String(), apiPort)
}
