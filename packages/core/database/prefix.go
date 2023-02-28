package database

const (
	// PrefixPeer defines the prefix of the peer db.
	PrefixPeer byte = iota

	// PrefixLedger defines the storage prefix for the ledger package.
	PrefixLedger

	// PrefixIndexer defines the storage prefix for the indexer package.
	PrefixIndexer
)
