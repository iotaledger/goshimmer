package framework

const (
	autopeeringMaxTries = 50

	apiPort = "8080"

	containerNameTester      = "/tester"
	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameDrand       = "drand_"
	containerNameSuffixPumba = "_pumba"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "portcheck,dashboard,analysis-client,profiling,gossip,drng,issuer,sync,metrics,valuetransfers,messagelayer,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint"
	disabledPluginsPeer      = "portcheck,dashboard,analysis-client,profiling"
	snapshotFilePath         = "/assets/7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin"
	dockerLogsPrefixLen      = 8

	dkgMaxTries = 50

	exitStatusSuccessful = 0
)

// Parameters to override before calling any peer creation function.
var (
	// ParaFCoBAverageNetworkDelay defines the configured avg. network delay (in seconds) for the FCOB rules.
	ParaFCoBAverageNetworkDelay = 5
	// ParaOutboundUpdateIntervalMs the autopeering outbound update interval in milliseconds.
	ParaOutboundUpdateIntervalMs = 100
	// ParaBootstrapOnEveryNode whether to enable the bootstrap plugin on every node.
	ParaBootstrapOnEveryNode = false
)

var (
	genesisSeed = []byte{95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113,
		104, 71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230}
)

//GoShimmerConfig defines the config of a GoShimmer node.
type GoShimmerConfig struct {
	Seed               string
	Name               string
	EntryNodeHost      string
	EntryNodePublicKey string
	DisabledPlugins    string
	SnapshotFilePath   string

	Bootstrap                             bool
	BootstrapInitialIssuanceTimePeriodSec int

	DRNGCommittee string
	DRNGDistKey   string
	DRNGInstance  int
	DRNGThreshold int

	Faucet bool
}

// NetworkConfig defines the config of a GoShimmer Docker network.
type NetworkConfig struct {
	BootstrapInitialIssuanceTimePeriodSec int
}
