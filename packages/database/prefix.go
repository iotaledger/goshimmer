package database

const (
	// PrefixPeer defines the prefix of the peer db.
	PrefixPeer byte = iota

	// PrefixHealth defines the prefix of the health db.
	PrefixHealth

	// PrefixTangle defines the storage prefix for the tangle.
	PrefixTangle

	// PrefixMarkers defines the storage prefix for the markers used to optimize structural checks in the tangle.
	PrefixMarkers

	// PrefixLedger defines the storage prefix for the ledger package.
	PrefixLedger

	// PrefixIndexer defines the storage prefix for the indexer package.
	PrefixIndexer

	// PrefixConflictDAG defines the storage prefix for the branchDAG package.
	PrefixConflictDAG

	// PrefixMana defines the storage prefix for the mana package.
	PrefixMana

	// PrefixEpochs defines the storage prefix for the epochs package.
	PrefixEpochs
)
