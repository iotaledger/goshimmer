package libp2putil

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
)

// GetLibp2pIdentity returns libp2p Host option for Identity from local peer object.
func GetLibp2pIdentity(lPeer *peer.Local) (libp2p.Option, error) {
	ourPrivateKey, err := lPeer.Database().LocalPrivateKey()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	libp2pPrivateKey, err := ToLibp2pPrivateKey(ourPrivateKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return libp2p.Identity(libp2pPrivateKey), nil
}

// ToLibp2pPrivateKey transforms private key in our type to libp2p type.
func ToLibp2pPrivateKey(ourPrivateKey ed25519.PrivateKey) (libp2pcrypto.PrivKey, error) {
	libp2pPrivateKey, err := libp2pcrypto.UnmarshalEd25519PrivateKey(ourPrivateKey.Bytes())
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return libp2pPrivateKey, nil
}

// ToLibp2pPeerID computes libp2p peer ID from our peer object.
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
