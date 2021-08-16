package dashboard

import (
	flag "github.com/spf13/pflag"
)

// type ParametersDefinition struct {
// 	// CfgBindAddress defines the config flag of the analysis dashboard binding address.
// 	CfgBindAddress string `default:"0.0.0.0:8000" usage:"the bind address of the analysis dashboard"`
// 	= "analysis.dashboard.bindAddress"
// 	// CfgDev defines the config flag of the analysis dashboard dev mode.
// 	CfgDev bool `default:"false" usage:""whether the analysis dashboard runs in dev mode""`
// 	= "analysis.dashboard.dev"
// 	// CfgBasicAuthEnabled defines the config flag of the analysis dashboard basic auth enabler.
// 	CfgBasicAuthEnabled bool `default:"false" usage:"whether to enable HTTP basic auth"`
// 	= "analysis.dashboard.basic_auth.enabled"
// 	// CfgBasicAuthUsername defines the config flag of the analysis dashboard basic auth username.
// 	CfgBasicAuthUsername string `default:"goshimmer" usage:"HTTP basic auth username"`
// 	= "analysis.dashboard.basic_auth.username"
// 	// CfgBasicAuthPassword defines the config flag of the analysis dashboard basic auth password.
// 	CfgBasicAuthPassword string `default:"goshimmer" usage:"HTTP basic auth password"`
// 	= "analysis.dashboard.basic_auth.password"
// 	// CfgMongoDBEnabled defines the config flag of the analysis dashboard to enable mongoDB.
// 	CfgMongoDBEnabled bool `default:"false" usage:"whether to enable MongoDB"`
// 	= "analysis.dashboard.mongodb.enabled"
// 	// CfgMongoDBUsername defines the config flag of the analysis dashboard mongoDB username.
// 	CfgMongoDBUsername string `default:"root" usage:"MongoDB username"`
// 	= "analysis.dashboard.mongodb.username"
// 	// CfgMongoDBPassword defines the config flag of the analysis dashboard mongoDB password.
// 	CfgMongoDBPassword string `default:"password" usage:"MongoDB username"`
// 	= "analysis.dashboard.mongodb.password"
// 	// CfgMongoDBHostAddress defines the config flag of the analysis dashboard mongoDB binding address.
// 	CfgMongoDBHostAddress string `default:"mongodb:27017" usage:"MongoDB host address"`
// 	= "analysis.dashboard.mongodb.hostAddress"
// 	// CfgManaDashboardAddress defines the address of the mana dashboard to stream mana info from.
// 	CfgManaDashboardAddress string `default:"http://127.0.0.1:8081" usage:"dashboard host address"`
// 	= "analysis.dashboard.manaAddress"
// 	}

const (
	// CfgBindAddress defines the config flag of the analysis dashboard binding address.
	CfgBindAddress = "analysis.dashboard.bindAddress"
	// CfgDev defines the config flag of the analysis dashboard dev mode.
	CfgDev = "analysis.dashboard.dev"
	// CfgBasicAuthEnabled defines the config flag of the analysis dashboard basic auth enabler.
	CfgBasicAuthEnabled = "analysis.dashboard.basic_auth.enabled"
	// CfgBasicAuthUsername defines the config flag of the analysis dashboard basic auth username.
	CfgBasicAuthUsername = "analysis.dashboard.basic_auth.username"
	// CfgBasicAuthPassword defines the config flag of the analysis dashboard basic auth password.
	CfgBasicAuthPassword = "analysis.dashboard.basic_auth.password"
	// CfgMongoDBEnabled defines the config flag of the analysis dashboard to enable mongoDB.
	CfgMongoDBEnabled = "analysis.dashboard.mongodb.enabled"
	// CfgMongoDBUsername defines the config flag of the analysis dashboard mongoDB username.
	CfgMongoDBUsername = "analysis.dashboard.mongodb.username"
	// CfgMongoDBPassword defines the config flag of the analysis dashboard mongoDB password.
	CfgMongoDBPassword = "analysis.dashboard.mongodb.password"
	// CfgMongoDBHostAddress defines the config flag of the analysis dashboard mongoDB binding address.
	CfgMongoDBHostAddress = "analysis.dashboard.mongodb.hostAddress"
	// CfgManaDashboardAddress defines the address of the mana dashboard to stream mana info from.
	CfgManaDashboardAddress = "analysis.dashboard.manaAddress"
)

func init() {
	flag.String(CfgBindAddress, "0.0.0.0:8000", "the bind address of the analysis dashboard")
	flag.Bool(CfgDev, false, "whether the analysis dashboard runs in dev mode")
	flag.Bool(CfgBasicAuthEnabled, false, "whether to enable HTTP basic auth")
	flag.String(CfgBasicAuthUsername, "goshimmer", "HTTP basic auth username")
	flag.String(CfgBasicAuthPassword, "goshimmer", "HTTP basic auth password")
	flag.Bool(CfgMongoDBEnabled, false, "whether to enable MongoDB")
	flag.String(CfgMongoDBUsername, "root", "MongoDB username")
	flag.String(CfgMongoDBPassword, "password", "MongoDB username")
	flag.String(CfgMongoDBHostAddress, "mongodb:27017", "MongoDB host address")
	flag.String(CfgManaDashboardAddress, "http://127.0.0.1:8081", "dashboard host address")
}

// // Parameters contains the configuration parameters of the logger plugin.
// var Parameters = &ParametersDefinition{}

// func init() {
// 	configuration.BindParameters(Parameters, "dashboard")
// }
