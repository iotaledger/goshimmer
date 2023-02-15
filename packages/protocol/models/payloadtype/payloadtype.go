package payloadtype

const (
	// GenericData is the generic data payload type.
	GenericData uint32 = iota

	// Transaction is the transaction payload type.
	Transaction

	// MockedTransaction is the mocked transaction payload type.
	MockedTransaction

	// FaucetRequest is the faucet request payload type.
	FaucetRequest
)
