package payload

import "github.com/mr-tron/base58"

type Id [IdLength]byte

func (id Id) Bytes() []byte {
	return id[:]
}

func (id Id) String() string {
	return base58.Encode(id[:])
}

const IdLength = 64
