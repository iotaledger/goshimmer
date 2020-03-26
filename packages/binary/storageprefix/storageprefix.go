package storageprefix

var (
	MainNet = []byte{0}

	TangleTransaction         = []byte{1}
	TangleTransactionMetadata = []byte{2}
	TangleApprovers           = []byte{3}
	TangleMissingTransaction  = []byte{4}

	ValueTransferPayload         = []byte{5}
	ValueTransferPayloadMetadata = []byte{6}
	ValueTransferApprover        = []byte{7}
	ValueTransferMissingPayload  = []byte{8}
	ValueTransferAttachment      = []byte{9}
	ValueTransferConsumer        = []byte{10}

	LedgerStateTransferOutput        = []byte{11}
	LedgerStateTransferOutputBooking = []byte{12}
	LedgerStateReality               = []byte{13}
	LedgerStateConflictSet           = []byte{14}
)
