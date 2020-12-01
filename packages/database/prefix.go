package database

const (
	// PrefixAutoPeering defines the prefix of the autopeering db.
	PrefixAutoPeering byte = iota
	// PrefixHealth defines the prefix of the health db.
	PrefixHealth
	// PrefixMessageLayer defines the storage prefix for the message layer.
	PrefixMessageLayer
	// PrefixMarker defines the storage prefix for the markers used to optimize structural checks in the tangle.
	PrefixMarker
	// PrefixValueTransfers defines the storage prefix for value transfer.
	PrefixValueTransfers
)
