package manaeventlogger

import flag "github.com/spf13/pflag"

const (
	// CfgCSV is the path to a csv file to store mana events.
	CfgCSV = "manaeventlogger.csv"
	// CfgBufferSize is the buffer size to store events.
	CfgBufferSize = "manaeventlogger.bufferSize"
	// CfgCheckBufferIntervalSec is the interval to check the events buffer size.
	CfgCheckBufferIntervalSec = "manaeventloger.checkBufferIntervalSec"
)

func init() {
	flag.String(CfgCSV, "/tmp/consensusManaEvents.csv", "file to store mana events")
	flag.Int(CfgBufferSize, 100, "event logs buffer size")
	flag.Int(CfgCheckBufferIntervalSec, 5, "check buffer interval secs")
}
