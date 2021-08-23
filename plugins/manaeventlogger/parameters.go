package manaeventlogger

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the mana event logger plugin.
type ParametersDefinition struct {
	// CSV defines the file path to store mana events.
	CSV string `default:"/tmp/consensusManaEvents.csv" usage:"file to store mana events"`
	// Buffersize defines the events' buffer size.
	BufferSize int `default:"100" usage:"event logs buffer size"`
	// CheckBufferInterval defines interval between buffer checks.
	CheckBufferInterval time.Duration `default:"5s" usage:"check buffer interval"`
}

// Parameters contains the configuration used by the mana event logger plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "manaEventLogger")
}
