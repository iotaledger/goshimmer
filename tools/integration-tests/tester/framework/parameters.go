package framework

import (
	"time"
)

const (
	peeringMaxTries     = 50
	waitForManaMaxTries = 50

	apiPort = "8080"

	containerNameTester      = "/tester"
	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameDrand       = "drand_"
	containerNameSuffixPumba = "_pumba"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "portcheck,dashboard,analysis-client,profiling,gossip,drng,issuer,metrics,valuetransfers,consensus,messagelayer,mana,pow,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint,clock"
	disabledPluginsPeer      = "portcheck,dashboard,analysis-client,profiling,clock"
	enabledPluginsPeer       = "WebAPI tools Endpoint"
	snapshotFilePath         = "/assets/7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin"
	dockerLogsPrefixLen      = 8

	dkgMaxTries = 50

	exitStatusSuccessful = 0

	syncBeaconSeed      = "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E"
	syncBeaconPublicKey = "CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3"

	// GenesisTokenAmount is the amount of tokens in the genesis output.
	GenesisTokenAmount = 1000000000000000
)

// Parameters to override before calling any peer creation function.
var (
	// ParaTangleTimeWindow defines the time window in which the node will consider itself in sync.
	ParaTangleTimeWindow = 30 * time.Second
	// ParaFCoBQuarantineTime defines the configured expected upper bound for messages being delivered, and is used for the FCOB rules.
	ParaFCoBQuarantineTime time.Duration = 5 * time.Second
	// DefaultUpperBoundNetworkDelay contains the default upper bound duration it takes for a message to propagate through the network through gossip.
	DefaultUpperBoundNetworkDelay time.Duration = 5 * time.Second
	// ParaOutboundUpdateIntervalMs the autopeering outbound update interval in milliseconds.
	ParaOutboundUpdateIntervalMs = 100
	// ParaFaucetTokensPerRequest defines the tokens to send up on each faucet request message.
	ParaFaucetTokensPerRequest int64 = 1000000
	// ParaPoWDifficulty defines the PoW difficulty.
	ParaPoWDifficulty = 2
	// ParaWaitToKill defines the time to wait before killing the node.
	ParaWaitToKill = 60
	// ParaPoWFaucetDifficulty defines the PoW difficulty for faucet payloads.
	ParaPoWFaucetDifficulty = 2
	// ParaFaucetPreparedOutputsCount defines the number of outputs the faucet should prepare.
	ParaFaucetPreparedOutputsCount = 10
	// ParaManaOnEveryNode defines whether all nodes should have mana enabled.
	ParaManaOnEveryNode = true
	// ParaFPCRoundInterval defines how long a round lasts (in seconds)
	ParaFPCRoundInterval int64 = 10
	// ParaFPCTotalRoundsFinalization the amount of FPC rounds where an opinion needs to stay the same to be considered final. Also called 'l'.
	ParaFPCTotalRoundsFinalization int = 10
	// ParaWaitForStatement is the time in seconds for which the node wait for receiving the new statement.
	ParaWaitForStatement = 3
	// ParaFPCListen defines if the FPC service should listen.
	ParaFPCListen = false
	// ParaWriteStatement defines if the node should write statements.
	ParaWriteStatement = true
	// ParaReadManaThreshold defines the Mana threshold to accept a statement.
	ParaReadManaThreshold = 1.0
	// ParaWriteManaThreshold defines the Mana threshold to write a statement.
	ParaWriteManaThreshold = 1.0
	// ParaSnapshotResetTime defines if the aMana Snapshot should be reset to the current Time.
	ParaSnapshotResetTime = false
	// ParaActivityInterval defines the interval between activity messages (in seconds).
	ParaActivityInterval = 1
	// ParaActivityPluginOnEveryNode defines if activity messages are issue by all the nodes.
	ParaActivityPluginOnEveryNode = false
)

var (
	genesisSeed = []byte{
		95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113,
		104, 71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230,
	}
	genesisSeedBase58 = "7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih"
)

// GoShimmerConfig defines the config of a GoShimmer node.
type GoShimmerConfig struct {
	Seed               string
	Name               string
	EntryNodeHost      string
	EntryNodePublicKey string
	DisabledPlugins    string
	EnabledPlugins     string
	SnapshotFilePath   string

	StartSynced bool

	DRNGCommittee string
	DRNGDistKey   string
	DRNGInstance  int
	DRNGThreshold int

	Faucet bool

	EnableAutopeeringForGossip bool

	ActivityPlugin   bool
	ActivityInterval int

	Mana                              bool
	ManaAllowedAccessFilterEnabled    bool
	ManaAllowedConsensusFilterEnabled bool
	ManaAllowedAccessPledge           []string
	ManaAllowedConsensusPledge        []string

	FPCRoundInterval           int64
	FPCTotalRoundsFinalization int
	WaitForStatement           int
	FPCListen                  bool
	WriteStatement             bool
	WriteManaThreshold         float64
	ReadManaThreshold          float64
	SnapshotResetTime          bool
}

// NetworkConfig defines the config of a GoShimmer Docker network.
type NetworkConfig struct {
	BootstrapInitialIssuanceTimePeriodSec int
}

// CreateNetworkConfig is the config for optional plugins passed through createNetwork.
type CreateNetworkConfig struct {
	Faucet      bool
	Mana        bool
	StartSynced bool
}
