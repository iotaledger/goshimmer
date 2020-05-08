package framework

const (
	autopeeringMaxTries = 50

	apiPort = "8080"

	containerNameTester    = "/tester"
	containerNameEntryNode = "entry_node"
	containerNameReplica   = "replica_"
	containerNameDrand     = "drand_"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "portcheck,dashboard,analysis,gossip,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint"
	disabledPluginsPeer      = "portcheck,dashboard,analysis"

	dockerLogsPrefixLen = 8

	dkgMaxTries = 50
)

// GoShimmerConfig defines the config of a goshimmer node.
type GoShimmerConfig struct {
	Seed               string
	Name               string
	EntryNodeHost      string
	EntryNodePublicKey string
	Bootstrap          bool
	DisabledPlugins    string

	DRNGCommittee string
	DRNGDistKey   string
	DRNGInstance  int
	DRNGThreshold int
}
