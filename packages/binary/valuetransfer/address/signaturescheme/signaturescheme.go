package signaturescheme

import (
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
)

// SignatureScheme defines an interface for different signature generation methods (i.e. ED25519, WOTS, and so on ...).
type SignatureScheme interface {
	// Version returns the version byte that is associated to this signature scheme.
	Version() byte

	// Address returns the address that this signature scheme instance is securing.
	Address() address.Address

	// Sign creates a valid signature for the given data according to the signature scheme implementation.
	Sign(data []byte) Signature
}
