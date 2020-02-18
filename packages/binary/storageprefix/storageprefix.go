package storageprefix

var (
	MainNet = []byte{0}

	TangleTransaction         = []byte{1}
	TangleTransactionMetadata = []byte{2}
	TangleApprovers           = []byte{3}
	TangleMissingTransaction  = []byte{4}

	ValueTangleTransferMetadata = []byte{5}
	ValueTangleConsumers        = []byte{6}
	ValueTangleMissingTransfers = []byte{7}

	LedgerStateTransferOutput        = []byte{8}
	LedgerStateTransferOutputBooking = []byte{9}
	LedgerStateReality               = []byte{10}
	LedgerStateConflictSet           = []byte{11}
)
