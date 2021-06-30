package entitylogging

import "time"

//

type BranchLogEntry struct {
	BranchID   string
	Timestamp time.Time
}

type LogEntry struct {
	Key string
	Values []interface
}
