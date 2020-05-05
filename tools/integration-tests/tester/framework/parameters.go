package framework

const (
	autopeeringMaxTries = 50

	apiPort = "8080"

	containerNameTester    = "/tester"
	containerNameEntryNode = "entry_node"
	containerNameReplica   = "replica_"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "portcheck,dashboard,analysis,gossip,webapi,webapibroadcastdataendpoint,webapifindtransactionhashesendpoint,webapigetneighborsendpoint,webapigettransactionobjectsbyhashendpoint,webapigettransactiontrytesbyhashendpoint"
	disabledPluginsPeer      = "portcheck,dashboard,analysis"

	dockerLogsPrefixLen = 8
)
