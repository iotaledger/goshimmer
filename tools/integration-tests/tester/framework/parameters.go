package framework

import (
	"fmt"
	"time"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
)

const (
	// ports
	apiPort       = 8080
	dagVizPort    = 8061
	dashboardPort = 8081
	gossipPort    = 14666
	peeringPort   = 14626

	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameDrand       = "drand_"
	containerNameSuffixPumba = "_pumba"

	graceTimePumba = 3 * time.Second

	logsDir             = "/tmp/logs/"
	dockerLogsPrefixLen = 8
)

var (
	// GenesisSeedBytes is the seed of the funds created at genesis.
	GenesisSeedBytes = []byte{
		95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113, 104,
		71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230,
	}
)

// CreateNetworkConfig is the config for optional plugins passed through NewNetwork.
type CreateNetworkConfig struct {
	// StartSynced specifies whether all node in the network start synced.
	StartSynced bool
	// Autopeering specifies whether autopeering or manual peering is used.
	Autopeering bool
	// Faucet specifies whether the first peer should have the faucet enabled.
	Faucet bool
	// PeerMaster specifies whether the network should include the peer master.
	PeerMaster bool
	// Activity specifies whether nodes schedule activity messages in regular intervals.
	Activity bool
	// Snapshots to be generated and rendered available for the network.
	Snapshots []SnapshotInfo
}

// PeerConfig specifies the default config of a standard GoShimmer peer.
func PeerConfig() config.GoShimmer {
	c := config.NewGoShimmer()

	c.Image = "iotaledger/goshimmer"

	c.DisabledPlugins = []string{"portcheck", "analysisClient", "profiling", "clock", "remotelogmetrics", "remotemetrics"}

	c.Network.Enabled = true

	c.Dashboard.Enabled = true
	c.Dashboard.BindAddress = fmt.Sprintf("0.0.0.0:%d", dashboardPort)

	c.Dagsvisualizer.Enabled = true
	c.Dagsvisualizer.BindAddress = fmt.Sprintf("0.0.0.0:%d", dagVizPort)

	c.Database.Enabled = true
	c.Database.ForceCacheTime = 0 // disable caching for tests

	c.Gossip.Enabled = true

	c.POW.Enabled = true
	c.POW.Difficulty = 1

	c.WebAPI.Enabled = true
	c.WebAPI.BindAddress = fmt.Sprintf(":%d", apiPort)

	c.AutoPeering.Enabled = false
	c.AutoPeering.BindAddress = fmt.Sprintf(":%d", peeringPort)
	c.AutoPeering.EntryNodes = nil

	c.MessageLayer.Enabled = true
	c.MessageLayer.Snapshot.File = fmt.Sprintf("/assets/%s.bin", base58.Encode(GenesisSeedBytes))
	c.MessageLayer.Snapshot.GenesisNode = "" // use the default time based approach

	c.Faucet.Enabled = false
	c.Faucet.Seed = base58.Encode(GenesisSeedBytes)
	c.Faucet.PowDifficulty = 1
	c.Faucet.SupplyOutputsCount = 4
	c.Faucet.SplittingMultiplier = 4
	c.Faucet.GenesisTokenAmount = 2500000000000000

	c.Mana.Enabled = true
	c.Mana.SnapshotResetTime = true

	c.Consensus.Enabled = false

	c.Activity.Enabled = false
	c.Activity.BroadcastInterval = time.Second // increase frequency to speedup tests

	c.DRNG.Enabled = false

	return c
}

// EntryNodeConfig specifies the default config of a standard GoShimmer entry node.
func EntryNodeConfig() config.GoShimmer {
	c := PeerConfig()

	c.DisabledPlugins = append(c.DisabledPlugins, "issuer", "metrics", "valuetransfers", "consensus", "manarefresher", "manualpeering", "chat",
		"WebAPIDataEndpoint", "WebAPIDRNGEndpoint", "WebAPIFaucetEndpoint", "WebAPIMessageEndpoint", "Snapshot", "WebAPIToolsDRNGEndpoint",
		"WebAPIToolsMessageEndpoint", "WebAPIWeightProviderEndpoint", "WebAPIInfoEndpoint", "WebAPILedgerstateEndpoint", "Firewall", "remotelog", "remotelogmetrics",
		"DAGsVisualizer")
	c.Gossip.Enabled = false
	c.POW.Enabled = false
	c.AutoPeering.Enabled = true
	c.MessageLayer.Enabled = false
	c.Faucet.Enabled = false
	c.Mana.Enabled = false
	c.Consensus.Enabled = false
	c.Activity.Enabled = false
	c.DRNG.Enabled = false
	c.Dashboard.Enabled = false
	c.Dagsvisualizer.Enabled = false

	return c
}
