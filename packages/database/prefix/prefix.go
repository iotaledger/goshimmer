package prefix

const (
	// DBPrefixApprovers defines the prefix of the approvers db
	DBPrefixApprovers byte = iota
	// DBPrefixTransaction defines the prefix of the transaction db
	DBPrefixTransaction
	// DBPrefixBundle defines the prefix of the bundles db
	DBPrefixBundle
	// DBPrefixTransactionMetadata defines the prefix of the transaction metadata db
	DBPrefixTransactionMetadata
	// DBPrefixAddressTransactions defines the prefix of the address transactions db
	DBPrefixAddressTransactions
	// DBPrefixAutoPeering defines the prefix of the autopeering db.
	DBPrefixAutoPeering
	// DBPrefixHealth defines the prefix of the health db.
	DBPrefixHealth
)
