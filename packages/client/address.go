package client

import (
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
)

type Address struct {
	trytes        trinary.Trytes
	securityLevel consts.SecurityLevel
	privateKey    trinary.Trits
}

func NewAddress(trytes trinary.Trytes) *Address {
	return &Address{
		trytes: trytes,
	}
}

func (address *Address) GetTrytes() trinary.Trytes {
	return address.trytes
}

func (address *Address) GetSecurityLevel() consts.SecurityLevel {
	return address.securityLevel
}

func (address *Address) GetPrivateKey() trinary.Trits {
	return address.privateKey
}
