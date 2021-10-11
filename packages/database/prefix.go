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

	// PrefixLedgerState defines the storage prefix for the ledgerstate package.
	PrefixLedgerState

	// PrefixMana defines the storage prefix for the mana package.
	PrefixMana

	// PrefixEpochs defines the storage prefix for the epochs package.
	PrefixEpochs
)
