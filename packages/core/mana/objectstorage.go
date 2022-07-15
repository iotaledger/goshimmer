package mana

const (
	// PrefixAccess is the storage prefix for access mana storage.
	PrefixAccess byte = iota

	// PrefixConsensus is the storage prefix for consensus mana storage.
	PrefixConsensus

	// PrefixAccessResearch is the storage prefix for research access mana storage.
	PrefixAccessResearch

	// PrefixConsensusResearch is the storage prefix for research consensus mana storage.
	PrefixConsensusResearch

	// PrefixEventStorage is the storage prefix for consensus mana event storage.
	PrefixEventStorage

	// PrefixConsensusPastVector is the storage prefix for consensus mana past vector storage.
	PrefixConsensusPastVector

	// PrefixConsensusPastMetadata is the storage prefix for consensus mana past vector metadata storage.
	PrefixConsensusPastMetadata
)
