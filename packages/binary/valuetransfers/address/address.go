package address

import (
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
)

type AddressVersion = byte

type AddressDigest = []byte

type Address [Length]byte

func New(bytes []byte) (address Address) {
	copy(address[:], bytes)

	return
}

func FromED25519PubKey(key ed25119.PublicKey) (address Address) {
	digest := blake2b.Sum256(key[:])

	address[0] = 0
	copy(address[1:], digest[:])

	return
}

func (address *Address) GetVersion() AddressVersion {
	return address[0]
}

func (address *Address) GetDigest() AddressDigest {
	return address[1:]
}

func (address Address) ToBytes() []byte {
	return address[:]
}

func (address Address) String() string {
	return "Address(" + base58.Encode(address.ToBytes()) + ")"
}

const Length = 33
