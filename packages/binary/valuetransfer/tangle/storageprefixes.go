package tangle

const (
	_ byte = iota
	PrefixPayload
	PrefixPayloadMetadata
	PrefixMissingPayload
	PrefixApprover
	PrefixAttachment
	PrefixOutput
	PrefixMissingOutput
	PrefixConsumer
)
