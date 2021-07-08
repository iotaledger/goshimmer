package local

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinitionLocal contains the definition of configuration parameters used by the local peer.
type ParametersDefinitionLocal struct {
	// Port defines the config flag of the autopeering port.
	Port int `default:"14626" usage:"UDP port for incoming peering requests"`

	// Seed defines the config flag of the autopeering private key seed.
	Seed string `usage:"private key seed used to derive the node identity; optional base58 or base64 encoded 256-bit string. Prefix with 'base58:' or 'base64', respectively"`
}

// ParametersDefinitionNetwork contains the definition of configuration parameters used by the local peer's network.
type ParametersDefinitionNetwork struct {
	// BindAddress defines the config flag of the network bind address.
	BindAddress string `default:"0.0.0.0" usage:"bind address for global services such as autopeering and gossip"`

	// ExternalAddress defines the config flag of the network external address.
	ExternalAddress string `default:"auto" usage:"external IP address under which the node is reachable; or 'auto' to determine it automatically"`
}

// ParametersLocal contains the configuration parameters of the local peer.
var ParametersLocal = &ParametersDefinitionLocal{}

// ParametersNetwork contains the configuration parameters of the local peer's network.
var ParametersNetwork = &ParametersDefinitionNetwork{}

func init() {
	configuration.BindParameters(ParametersLocal, "autopeering")
	configuration.BindParameters(ParametersNetwork, "network")
}
