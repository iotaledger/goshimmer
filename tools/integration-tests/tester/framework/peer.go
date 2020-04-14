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

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/plugins/webapi/autopeering"
)

// Peer represents a GoShimmer node inside the Docker network
type Peer struct {
	name string
	ip   net.IP
	*client.GoShimmerAPI
	dockerCli *dockerclient.Client
	chosen    []autopeering.Neighbor
	accepted  []autopeering.Neighbor
}

// NewPeer creates a new instance of Peer with the given information.
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

// TotalNeighbors returns the total number of neighbors the peer has.
func (p *Peer) TotalNeighbors() int {
	return len(p.chosen) + len(p.accepted)
}

// SetNeighbors sets the neighbors of the peer accordingly.
func (p *Peer) SetNeighbors(chosen, accepted []autopeering.Neighbor) {
	p.chosen = make([]autopeering.Neighbor, len(chosen))
	copy(p.chosen, chosen)

	p.accepted = make([]autopeering.Neighbor, len(accepted))
	copy(p.accepted, accepted)
}

// Logs returns the logs of the peer as io.ReadCloser.
// Logs are returned via Docker and contain every log entry since start of the container/GoShimmer node.
func (p *Peer) Logs() (io.ReadCloser, error) {
	return p.dockerCli.ContainerLogs(
		context.Background(),
		p.name,
		types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Since:      "",
			Timestamps: false,
			Follow:     false,
			Tail:       "",
			Details:    false,
		})
}

// getAvailablePeers gets all available peers in the Docker network.
// It uses the expected Docker hostnames and tries to resolve them.
// If that does not work it means the host is not available in the network
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

// getWebApiBaseUrl returns the web API base url for the given IP.
func getWebApiBaseUrl(ip net.IP) string {
	return fmt.Sprintf("http://%s:%s", ip.String(), apiPort)
}
