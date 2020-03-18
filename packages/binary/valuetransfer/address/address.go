package address

import (
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
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

func FromBytes(bytes []byte) (result Address, err error, consumedBytes int) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	addressBytes, err := marshalUtil.ReadBytes(Length)
	if err != nil {
		return
	}
	copy(result[:], addressBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse
func Parse(marshalUtil *marshalutil.MarshalUtil) (Address, error) {
	if address, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return Address{}, err
	} else {
		return address.(Address), nil
	}
}

func (address *Address) GetVersion() AddressVersion {
	return address[0]
}

func (address *Address) GetDigest() AddressDigest {
	return address[1:]
}

func (address Address) Bytes() []byte {
	return address[:]
}

func (address Address) String() string {
	return "Address(" + base58.Encode(address.Bytes()) + ")"
}

const Length = 33
