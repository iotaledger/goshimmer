package wallet

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// Seed represents a seed for IOTA wallets. A seed allows us to generate a deterministic sequence of Addresses and their
// corresponding KeyPairs.
type Seed struct {
	*ed25519.Seed
}

// NewSeed is the factory method for an IOTA seed. It either generates a new one or imports an existing  marshaled seed.
// before.
func NewSeed(optionalSeedBytes ...[]byte) *Seed {
	return &Seed{
		ed25519.NewSeed(optionalSeedBytes...),
	}
}

// Address returns an ed25519 address which can be used for receiving or sending funds.
func (seed *Seed) Address(index uint64) address.Address {
	return address.FromED25519PubKey(seed.Seed.KeyPair(index).PublicKey)
}
