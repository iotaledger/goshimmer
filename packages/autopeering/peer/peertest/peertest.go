// Package peertest provides utilities for writing tests with the peer package.
package peertest

import (
	"crypto/ed25519"
	"log"
	"math/rand"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
)

func NewPeer(network, address string) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, network, address)
	key := make([]byte, ed25519.PublicKeySize)
	copy(key, address)
	return peer.NewPeer(key, services)
}

func NewLocal(network, address string, db *peer.DB) *peer.Local {
	services := service.New()
	services.Update(service.PeeringKey, network, address)
	local, err := peer.NewLocal(services, db, randomSeed())
	if err != nil {
		log.Panic(err)
	}
	return local
}

func randomSeed() []byte {
	seed := make([]byte, ed25519.SeedSize)
	rand.Read(seed)
	return seed
}
