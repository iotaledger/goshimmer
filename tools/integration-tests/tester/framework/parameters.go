package framework

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
)

const (
	autopeeringMaxTries = 50

	apiPort = "8080"

	containerNameTester      = "/tester"
	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameDrand       = "drand_"
	containerNameSuffixPumba = "_pumba"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "portcheck,dashboard,analysis-client,profiling,gossip,drng,issuer,sync,metrics,valuetransfers,testsnapshots,messagelayer,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint"
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
	faucetSeed = []byte{251, 163, 190, 98, 92, 82, 164, 79, 74, 48, 203, 162, 247, 119, 140, 76, 33, 100, 148, 204, 244, 248, 232, 18,
		132, 217, 85, 31, 246, 83, 193, 193}
)

// GoShimmerConfig defines the config of a GoShimmer node.
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

	Wallet *wallet.Wallet
}

// NetworkConfig defines the config of a GoShimmer Docker network.
type NetworkConfig struct {
	BootstrapInitialIssuanceTimePeriodSec int
}
