package libp2putil

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
)

func ToLibp2pPrivateKey(ourPrivateKey ed25519.PrivateKey) (libp2pcrypto.PrivKey, error) {
	libp2pPrivateKey, err := libp2pcrypto.UnmarshalEd25519PrivateKey(ourPrivateKey.Bytes())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return libp2pPrivateKey, nil
}

func ToLibp2pPeerID(p *peer.Peer) (libp2ppeer.ID, error) {
	pubKeyBytes := p.PublicKey().Bytes()
	pubKeyLibp2p, err := libp2pcrypto.UnmarshalEd25519PublicKey(pubKeyBytes)
	if err != nil {
		return "", errors.WithStack(err)
	}
	libp2pID, err := libp2ppeer.IDFromPublicKey(pubKeyLibp2p)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return libp2pID, nil
}
