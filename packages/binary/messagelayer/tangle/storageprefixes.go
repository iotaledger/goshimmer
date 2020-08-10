package tangle

const (
	// PrefixMessage defines the storage prefix for message.
	PrefixMessage byte = iota
	// PrefixMessageMetadata defines the storage prefix for message metadata.
	PrefixMessageMetadata
	// PrefixApprovers defines the storage prefix for approvers.
	PrefixApprovers
	// PrefixMissingMessage defines the storage prefix for missing message.
	PrefixMissingMessage
)
