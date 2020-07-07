package server

import (
	"time"
)

// the period in which we scan and delete old data.
const cleanUpPeriod = 15 * time.Second
