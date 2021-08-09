package discovery

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinitionDiscovery contains the definition of configuration parameters used by the autopeering peer discovery.
type ParametersDefinitionDiscovery struct {
	// NetworkVersion defines the config flag of the network version.
	NetworkVersion int `default:"37" usage:"autopeering network version"`

	// EntryNodes defines the config flag of the entry nodes.
	EntryNodes []string `default:"2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626,5EDH4uY78EA6wrBkHHAVBWBMDt7EcksRq6pjzipoW15B@entryshimmer.tanglebay.com:14646" usage:"list of trusted entry nodes for auto peering"`
}

// Parameters contains the configuration parameters of the autopeering peer discovery.
var Parameters = &ParametersDefinitionDiscovery{}

func init() {
	configuration.BindParameters(Parameters, "autopeering")
}
