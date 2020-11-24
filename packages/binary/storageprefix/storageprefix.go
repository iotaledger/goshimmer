package storageprefix

const (
	// the following values are a list of prefixes defined as an enum
	// package specific prefixes used for the objectstorage in the corresponding packages

	_ byte = iota

	// MessageLayer defines the storage prefix for the message layer
	MessageLayer
	// ValueTransfers defines the storage prefix for value transfer.
	ValueTransfers
	// Mana defines the storage prefix for all mana.
	Mana
	// ManaAccess defines the storage prefix for access mana.
	ManaAccess
	// ManaConsensus defines the storage prefix for consensus mana.
	ManaConsensus
	// ManaAccessResearch defines the storage prefix for access research mana.
	ManaAccessResearch
	// ManaConsensusResearch defines the storage prefix for consensus research mana.
	ManaConsensusResearch
)
