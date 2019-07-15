package client

import (
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/signing"
	"github.com/iotaledger/iota.go/trinary"
)

type Seed struct {
	trytes        trinary.Trytes
	securityLevel consts.SecurityLevel
	subSeeds      map[uint64]trinary.Trits
}

func NewSeed(trytes trinary.Trytes, securityLevel consts.SecurityLevel) *Seed {
	return &Seed{
		trytes:        trytes,
		securityLevel: securityLevel,
		subSeeds:      make(map[uint64]trinary.Trits),
	}
}

func (seed *Seed) GetAddress(index uint64) *Address {
	addressTrytes, err := address.GenerateAddress(seed.trytes, index, seed.securityLevel)
	if err != nil {
		panic(err)
	}

	privateKey, err := signing.Key(seed.GetSubSeed(index), seed.securityLevel)
	if err != nil {
		panic(err)
	}

	return &Address{
		trytes:        addressTrytes,
		securityLevel: seed.securityLevel,
		privateKey:    privateKey,
	}
}

func (seed *Seed) GetSubSeed(index uint64) trinary.Trits {
	subSeed, subSeedExists := seed.subSeeds[index]
	if !subSeedExists {
		generatedSubSeed, err := signing.Subseed(seed.trytes, index)
		if err != nil {
			panic(err)
		}
		subSeed = generatedSubSeed

		seed.subSeeds[index] = subSeed
	}

	return subSeed
}
