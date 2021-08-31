package peer

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the local peer's network.
type ParametersDefinition struct {
	// Seed defines the config flag of the autopeering private key seed.
	Seed string `usage:"private key seed used to derive the node identity; optional base58 or base64 encoded 256-bit string. Prefix with 'base58:' or 'base64', respectively"`

	// ExternalAddress defines the config flag of the network external address.
	ExternalAddress string `default:"auto" usage:"external IP address under which the node is reachable; or 'auto' to determine it automatically"`
}

// Parameters contains the configuration parameters of the local peer's network.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "node")
}
