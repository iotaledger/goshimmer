package messagerequester

import (
	"time"
)

const (
	DefaultRequestWorkerCount = 1024
	DefaultRetryInterval      = 10 * time.Second
)
