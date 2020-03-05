package address

import (
	"github.com/mr-tron/base58"
)

type AddressVersion = byte

type AddressDigest = []byte

type Address [Length]byte

func New(bytes []byte) (address Address) {
	copy(address[:], bytes)

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
