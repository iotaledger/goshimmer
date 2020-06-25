package messagerequester

import (
	"runtime"
	"time"
)

const (
	DefaultRetryInterval = 10 * time.Second
)

var (
	DefaultRequestWorkerCount = runtime.GOMAXPROCS(0)
)
