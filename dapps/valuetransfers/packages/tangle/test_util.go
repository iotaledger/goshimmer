package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

// seed represents a seed for IOTA wallets. A seed allows us to generate a deterministic sequence of Addresses and their
// corresponding KeyPairs.
type seed struct {
	*ed25519.Seed
}

// newSeed is the factory method for an IOTA seed. It either generates a new one or imports an existing  marshaled seed.
// before.
func newSeed(optionalSeedBytes ...[]byte) *seed {
	return &seed{
		ed25519.NewSeed(optionalSeedBytes...),
	}
}

// Address returns an Address which can be used for receiving or sending funds.
func (seed *seed) Address(index uint64) address.Address {
	return address.FromED25519PubKey(seed.Seed.KeyPair(index).PublicKey)
}
