package database

const (
	// PrefixAutoPeering defines the prefix of the autopeering db.
	PrefixAutoPeering byte = iota

	// PrefixHealth defines the prefix of the health db.
	PrefixHealth

	// PrefixTangle defines the storage prefix for the tangle.
	PrefixTangle

	// PrefixMarkers defines the storage prefix for the markers used to optimize structural checks in the tangle.
	PrefixMarkers

	// PrefixLedgerState defines the storage prefix for the ledgerstate package.
	PrefixLedgerState

	// PrefixMana defines the storage prefix for the mana package.
	PrefixMana

	// PrefixFCOB defines the storage prefix for the fcob consensus package.
	PrefixFCOB

	// PrefixEpochs defines the storage prefix for the epochs package.
	PrefixEpochs
)
