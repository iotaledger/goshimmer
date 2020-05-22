package wallet

// Wallet represents a simple cryptocurrency wallet for the IOTA tangle. It contains the logic to manage the movement of
// funds.
type Wallet struct {
	seed *Seed
}

// New is the factory method of the wallet. It either creates a new wallet or restores the wallet backup that is handed
// in as an optional parameter.
func New(optionalRecoveryBytes ...[]byte) *Wallet {
	return &Wallet{
		seed: NewSeed(optionalRecoveryBytes...),
	}
}

// Seed returns the seed of this wallet that is used to generate all of the wallets addresses and private keys.
func (wallet *Wallet) Seed() *Seed {
	return wallet.seed
}
