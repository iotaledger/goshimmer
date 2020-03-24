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

	LedgerStateTransferOutput        = []byte{10}
	LedgerStateTransferOutputBooking = []byte{11}
	LedgerStateReality               = []byte{12}
	LedgerStateConflictSet           = []byte{13}
)
