package database

const (
	// PrefixAutoPeering defines the prefix of the autopeering db.
	PrefixAutoPeering byte = iota
	// PrefixHealth defines the prefix of the health db.
	PrefixHealth
	// PrefixMessageLayer defines the storage prefix for the message layer.
	PrefixMessageLayer
	// PrefixValueTransfers defines the storage prefix for value transfer.
	PrefixValueTransfers
)
