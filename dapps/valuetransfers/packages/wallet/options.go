package wallet

// Option represents an optional parameter .
type Option func(*Wallet)

// ImportSeed imports an existing seed into the wallet.
func ImportSeed(seedBytes []byte) Option {
	return func(wallet *Wallet) {
		wallet.seed = NewSeed(seedBytes)
	}
}

// SingleAddress configures the wallet to run in "single address" mode where all the funds are always managed on a
// single reusable address.
func SingleAddress(duration bool) Option {
	return func(wallet *Wallet) {
		wallet.singleAddress = duration
	}
}
