package client

import (
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
)

type ValueBundleFactory struct {
	seed          trinary.Trytes
	securityLevel consts.SecurityLevel
	seedInputs    map[uint64]uint64
	seedOutputs   map[uint64]uint64
}

func New(seed trinary.Trytes, securityLevel consts.SecurityLevel) *ValueBundleFactory {
	return &ValueBundleFactory{
		seed:          seed,
		securityLevel: securityLevel,
		seedInputs:    make(map[uint64]uint64),
		seedOutputs:   make(map[uint64]uint64),
	}
}

func (factory *ValueBundleFactory) AddInput(addressIndex uint64, value uint64) {
	factory.seedInputs[addressIndex] = value
}
