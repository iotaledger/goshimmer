package signaturescheme

import "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"

// Signature defines an interface for an address signature generated by the corresponding signature scheme.
type Signature interface {
	// IsValid returns true if the signature is valid for the given data.
	IsValid(signedData []byte) bool

	// Bytes returns a marshaled version of the signature.
	Bytes() []byte

	// Address returns the address, that this signature signs.
	Address() address.Address
}
