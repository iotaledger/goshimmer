package storageprefix

var (
	TangleTransaction         = []byte{0}
	TangleTransactionMetadata = []byte{1}
	TangleApprovers           = []byte{2}
	TangleMissingTransaction  = []byte{3}

	ValueTangleTransferMetadata = []byte{4}
	ValueTangleConsumers        = []byte{5}
	ValueTangleMissingTransfers = []byte{6}

	LedgerStateTransferOutput        = []byte{7}
	LedgerStateTransferOutputBooking = []byte{8}
	LedgerStateReality               = []byte{9}
	LedgerStateConflictSet           = []byte{10}
)
