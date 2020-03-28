package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	peerMasterHostname        = "peer_master"
	peerReplicaHostnamePrefix = "integration-tests_peer_replica_"
	getNeighborsAPI           = "getNeighbors"
	webApiPort                = "8080"
	minimumNeighbors          = 2
)

var (
	peersIp     []net.IP
	peersWebApi []string
)

func TestMain(m *testing.M) {
	fmt.Printf("Finding available peers...\n")
	peersIp = getAvailablePeers()
	peersWebApi = getPeersWebAPI(peersIp)
	fmt.Printf("Finding available peers... done. Peers: %v\n", peersIp)

	fmt.Printf("Waiting for autopeering...\n")
	waitForNeighbors(peersWebApi)
	fmt.Printf("Waiting for autopeering... done\n")

	// call the tests
	os.Exit(m.Run())
}

func TestExample(t *testing.T) {
	fmt.Println("This is a test")
}

func getAvailablePeers() (p []net.IP) {
	// get peer master
	if addr, err := net.LookupIP(peerMasterHostname); err != nil {
		fmt.Printf("Could not resolve %s\n", peerMasterHostname)
	} else {
		p = append(p, addr[0])
	}

	// get peer replicas
	for i := 1; ; i++ {
		peerName := fmt.Sprintf("%s%d", peerReplicaHostnamePrefix, i)
		if addr, err := net.LookupIP(peerName); err != nil {
			//fmt.Printf("Could not resolve %s\n", peerName)
			break
		} else {
			p = append(p, addr[0])
		}
	}

	if len(p) == 0 {
		panic("Could not find any peers in Docker network.")
	}

	return
}

func getPeersWebAPI(ips []net.IP) (apis []string) {
	for _, ip := range ips {
		url := fmt.Sprintf("http://%s:%s", ip.String(), webApiPort)
		apis = append(apis, url)
	}
	return
}

func waitForNeighbors(peers []string) {
	client := &http.Client{Timeout: 10 * time.Second}
	neighbors := make(map[string]int)

	maxTries := 50
	for maxTries > 0 {

		for _, base := range peers {
			resp := new(Response)
			url := fmt.Sprintf("%s/%s", base, getNeighborsAPI)
			if err := getJson(client, url, resp); err != nil {
				fmt.Printf("request error: %v\n", err)
				break
			}

			totalNeighbors := len(resp.Chosen) + len(resp.Accepted)
			neighbors[base] = totalNeighbors

			// if below threshold, quit current round
			if totalNeighbors < minimumNeighbors {
				break
			}
		}

		min := 0
		total := 0
		for _, num := range neighbors {
			if num > min {
				min = num
			}
			total += num
		}
		if min >= minimumNeighbors {
			fmt.Printf("Neighbors: min=%d avg=%.2f\n", min, float64(total)/float64(len(neighbors)))
			return
		}

		fmt.Println("Wait for 5 seconds...")
		time.Sleep(5 * time.Second)
		maxTries--
	}
}

func getJson(client *http.Client, url string, target interface{}) error {
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

type Response struct {
	KnownPeers []Neighbor `json:"known,omitempty"`
	Chosen     []Neighbor `json:"chosen"`
	Accepted   []Neighbor `json:"accepted"`
	Error      string     `json:"error,omitempty"`
}

type Neighbor struct {
	ID        string        `json:"id"`        // comparable node identifier
	PublicKey string        `json:"publicKey"` // public key used to verify signatures
	Services  []peerService `json:"services,omitempty"`
}

type peerService struct {
	ID      string `json:"id"`      // ID of the service
	Address string `json:"address"` // network address of the service
}
