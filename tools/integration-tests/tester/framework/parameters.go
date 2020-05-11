package framework

const (
	autopeeringMaxTries = 50

	apiPort = "8080"

	containerNameTester    = "/tester"
	containerNameEntryNode = "entry_node"
	containerNameReplica   = "replica_"
	containerNameDrand     = "drand_"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "portcheck,dashboard,analysis-client,gossip,drng,issuer,sync,metrics,messagelayer,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint"
	disabledPluginsPeer      = "portcheck,dashboard,analysis-client"

	dockerLogsPrefixLen = 8

	dkgMaxTries = 50
)

// GoShimmerConfig defines the config of a GoShimmer node.
type GoShimmerConfig struct {
	Seed               string
	Name               string
	EntryNodeHost      string
	EntryNodePublicKey string
	DisabledPlugins    string

	Bootstrap                             bool
	BootstrapInitialIssuanceTimePeriodSec int

	DRNGCommittee string
	DRNGDistKey   string
	DRNGInstance  int
	DRNGThreshold int
}

// NetworkConfig defines the config of a GoShimmer Docker network.
type NetworkConfig struct {
	BootstrapInitialIssuanceTimePeriodSec int
}
