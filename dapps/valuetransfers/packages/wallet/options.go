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

// GenericConnector allows us to provide a generic connector to the wallet. It can be used to mock the behavior of a
// real connector in tests or to provide new connection methods for nodes.
func GenericConnector(connector Connector) Option {
	return func(wallet *Wallet) {
		wallet.connector = connector
	}
}
