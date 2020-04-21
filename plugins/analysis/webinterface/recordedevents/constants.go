package recordedevents

import "time"

// CleanUpPeriod is the period in which we scan and delete old data
const CleanUpPeriod = 15 * time.Second
