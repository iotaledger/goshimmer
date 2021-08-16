package dashboard

import "github.com/iotaledger/hive.go/configuration"

type ParametersDefinition struct {
	// CfgBindAddress defines the config flag of the analysis dashboard binding address.
	CfgBindAddress string `default:"0.0.0.0:8000" usage:"the bind address of the analysis dashboard"`
	// CfgDev defines the config flag of the analysis dashboard dev mode.
	CfgDev bool `default:"false" usage:""whether the analysis dashboard runs in dev mode""`
	// CfgBasicAuthEnabled defines the config flag of the analysis dashboard basic auth enabler.
	CfgBasicAuthEnabled bool `default:"false" usage:"whether to enable HTTP basic auth"`
	// CfgBasicAuthUsername defines the config flag of the analysis dashboard basic auth username.
	CfgBasicAuthUsername string `default:"goshimmer" usage:"HTTP basic auth username"`
	// CfgBasicAuthPassword defines the config flag of the analysis dashboard basic auth password.
	CfgBasicAuthPassword string `default:"goshimmer" usage:"HTTP basic auth password"`
	// CfgMongoDBEnabled defines the config flag of the analysis dashboard to enable mongoDB.
	CfgMongoDBEnabled bool `default:"false" usage:"whether to enable MongoDB"`
	// CfgMongoDBUsername defines the config flag of the analysis dashboard mongoDB username.
	CfgMongoDBUsername string `default:"root" usage:"MongoDB username"`
	// CfgMongoDBPassword defines the config flag of the analysis dashboard mongoDB password.
	CfgMongoDBPassword string `default:"password" usage:"MongoDB username"`
	// CfgMongoDBHostAddress defines the config flag of the analysis dashboard mongoDB binding address.
	CfgMongoDBHostAddress string `default:"mongodb:27017" usage:"MongoDB host address"`
	// CfgManaDashboardAddress defines the address of the mana dashboard to stream mana info from.
	CfgManaDashboardAddress string `default:"http://127.0.0.1:8081" usage:"dashboard host address"`
}

// Parameters contains the configuration parameters of the logger plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "analysis.dashboard")
}
