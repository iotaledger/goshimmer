package discovery

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the message layer.
var Parameters = struct {
	// NetworkVersion defines the config flag of the network version.
	NetworkVersion int `default:"24" usage:"autopeering network version"`

	// EntryNodes defines the config flag of the entry nodes.
	EntryNodes []string `default:"2PV5487xMw5rasGBXXWeqSi4hLz7r19YBt8Y1TGAsQbj@ressims.iota.cafe:15626,5EDH4uY78EA6wrBkHHAVBWBMDt7EcksRq6pjzipoW15B@entryshimmer.tanglebay.com:14646" usage:"list of trusted entry nodes for auto peering"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "autopeering")
}
