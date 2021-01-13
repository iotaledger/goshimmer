package manaeventlogger

import flag "github.com/spf13/pflag"

const (
	// CfgCSV is the path to a csv file to store mana events.
	CfgCSV = "manaeventlogger.csv"
	// CfgBufferSize is the buffer size to store events.
	CfgBufferSize = "manaeventlogger.bufferSize"
)

func init() {
	flag.String(CfgCSV, "/tmp/consensusManaEvents.csv", "file to store mana events")
	flag.Int(CfgBufferSize, 100, "event logs buffer size")
}
