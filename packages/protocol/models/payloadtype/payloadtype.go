package payloadtype

const (
	// GenericData is the generic data payload type.
	GenericData uint32 = iota

	// Transaction is the transaction payload type.
	Transaction

	// MockedTransaction is the mocked transaction payload type.
	MockedTransaction

	// NetworkDelay is the network delay payload type.
	NetworkDelay

	// FaucetRequest is the faucet request payload type.
	FaucetRequest

	// Chat is the chat payload type.
	Chat
)
