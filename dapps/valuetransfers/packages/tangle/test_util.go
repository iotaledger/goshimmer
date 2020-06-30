package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

type seed struct {
	*ed25519.Seed
}

func newSeed(optionalSeedBytes ...[]byte) *seed {
	return &seed{
		ed25519.NewSeed(optionalSeedBytes...),
	}
}

func (seed *seed) Address(index uint64) address.Address {
	return address.FromED25519PubKey(seed.Seed.KeyPair(index).PublicKey)
}
