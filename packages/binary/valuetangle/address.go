package valuetangle

type AddressVersion = byte

type AddressDigest = []byte

type Address [AddressLength]byte

func NewAddress(bytes []byte) (address Address) {
	copy(address[:], bytes)

	return
}

func (address *Address) GetVersion() AddressVersion {
	return address[0]
}

func (address *Address) GetDigest() AddressDigest {
	return address[1:]
}

const AddressLength = 33
