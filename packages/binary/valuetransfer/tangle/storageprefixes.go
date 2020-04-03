package tangle

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	PrefixPayload
	PrefixPayloadMetadata
	PrefixMissingPayload
	PrefixApprover
	PrefixAttachment
	PrefixOutput
	PrefixMissingOutput
	PrefixConsumer
)
