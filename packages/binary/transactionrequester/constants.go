package transactionrequester

import (
	"time"
)

const (
	DEFAULT_REQUEST_WORKER_COUNT = 1024
	DEFAULT_RETRY_INTERVAL       = 10 * time.Second
)
