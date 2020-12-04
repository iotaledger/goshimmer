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

	disabledPluginsEntryNode = "portcheck,dashboard,analysis-client,profiling,gossip,drng,issuer,syncbeaconfollower,metrics,valuetransfers,consensus,messagelayer,pow,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint"
	disabledPluginsPeer      = "portcheck,dashboard,analysis-client,profiling"
	snapshotFilePath         = "/assets/7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin"
	dockerLogsPrefixLen      = 8

	dkgMaxTries = 50

	exitStatusSuccessful = 0

	syncBeaconSeed      = "Dw6dKWvQGbcijpib6A8t1vSiuDU1XWsnT71xhLSzXUGc"
	syncBeaconPublicKey = "6wuo4zNP4MXzojmj2EXGsPEHPkWJNnbKZ9e17ufdTmp"
)

// Parameters to override before calling any peer creation function.
var (
	// ParaFCoBAverageNetworkDelay defines the configured avg. network delay (in seconds) for the FCOB rules.
	ParaFCoBAverageNetworkDelay = 5
	// ParaOutboundUpdateIntervalMs the autopeering outbound update interval in milliseconds.
	ParaOutboundUpdateIntervalMs = 100
	// ParaFaucetTokensPerRequest defines the tokens to send up on each faucet request message.
	ParaFaucetTokensPerRequest int64 = 1337
	// ParaPoWDifficulty defines the PoW difficulty.
	ParaPoWDifficulty = 2
	// ParaWaitToKill defines the time to wait before killing the node.
	ParaWaitToKill = 60
	// ParaPoWFaucetDifficulty defines the PoW difficulty for faucet payloads.
	ParaPoWFaucetDifficulty = 2
	// ParaSyncBeaconOnEveryNode defines whether all nodes should be sync beacons.
	ParaSyncBeaconOnEveryNode = false
	// ParaFPCRoundInterval defines how long a round lasts (in seconds)
	ParaFPCRoundInterval int64 = 5
	// ParaWaitForStatement is the time in seconds for which the node wait for receiveing the new statement.
	ParaWaitForStatement = 3
	// ParaFPCListen defines if the FPC service should listen.
	ParaFPCListen = false
)

var (
	genesisSeed = []byte{95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113,
		104, 71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230}
	genesisSeedBase58 = "7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih"
)

//GoShimmerConfig defines the config of a GoShimmer node.
type GoShimmerConfig struct {
	Seed               string
	Name               string
	EntryNodeHost      string
	EntryNodePublicKey string
	DisabledPlugins    string
	SnapshotFilePath   string

	DRNGCommittee string
	DRNGDistKey   string
	DRNGInstance  int
	DRNGThreshold int

	Faucet bool

	SyncBeacon                  bool
	SyncBeaconFollower          bool
	SyncBeaconFollowNodes       string
	SyncBeaconBroadcastInterval int
	SyncBeaconMaxTimeOfflineSec int

	FPCRoundInterval int64
	WaitForStatement int
	FPCListen        bool
}

// NetworkConfig defines the config of a GoShimmer Docker network.
type NetworkConfig struct {
	BootstrapInitialIssuanceTimePeriodSec int
}
