package storageprefix

var (
	MainNet = []byte{0}

	Layer0Message         = []byte{1}
	Layer0MessageMetadata = []byte{2}
	Layer0Approvers       = []byte{3}
	Layer0MissingMessage  = []byte{4}

	ValueTransferPayload         = []byte{5}
	ValueTransferPayloadMetadata = []byte{6}
	ValueTransferApprover        = []byte{7}
	ValueTransferMissingPayload  = []byte{8}
	ValueTransferAttachment      = []byte{9}
	ValueTransferConsumer        = []byte{10}
	ValueTangleOutputs           = []byte{11}

	LedgerStateTransferOutput        = []byte{12}
	LedgerStateTransferOutputBooking = []byte{13}
	LedgerStateReality               = []byte{14}
	LedgerStateConflictSet           = []byte{15}
)
