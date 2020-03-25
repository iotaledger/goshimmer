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

	LedgerStateTransferOutput        = []byte{9}
	LedgerStateTransferOutputBooking = []byte{10}
	LedgerStateReality               = []byte{11}
	LedgerStateConflictSet           = []byte{12}
)
