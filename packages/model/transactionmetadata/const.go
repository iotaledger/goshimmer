package transactionmetadata

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

const (
	MARSHALED_HASH_START          = 0
	MARSHALED_RECEIVED_TIME_START = MARSHALED_HASH_END
	MARSHALED_FLAGS_START         = MARSHALED_RECEIVED_TIME_END

	MARSHALED_HASH_END          = MARSHALED_HASH_START + MARSHALED_HASH_SIZE
	MARSHALED_RECEIVED_TIME_END = MARSHALED_RECEIVED_TIME_START + MARSHALED_RECEIVED_TIME_SIZE
	MARSHALED_FLAGS_END         = MARSHALED_FLAGS_START + MARSHALED_FLAGS_SIZE

	MARSHALED_HASH_SIZE          = 81
	MARSHALED_RECEIVED_TIME_SIZE = 15
	MARSHALED_FLAGS_SIZE         = 1

	MARSHALED_TOTAL_SIZE = MARSHALED_FLAGS_END
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
