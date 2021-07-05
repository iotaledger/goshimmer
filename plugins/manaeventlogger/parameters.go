package manaeventlogger

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the manaeventlogger plugin.
type ParametersDefinition struct {
	// CSV defines the file path to store mana events.
	CSV string `name:"csv" default:"/tmp/consensusManaEvents.csv" usage:"file to store mana events"`
	// Buffersize defines the events' buffer size.
	BufferSize int `default:"100" usage:"event logs buffer size"`
	// CheckBufferIntervalSec defines interval between buffer checks.
	CheckBufferIntervalSec int `default:"5" usage:"check buffer interval secs"`
}

// Parameters contains the configuration used by the manaeventlogger plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "manaeventlogger")
}
