package dashboard

import (
	flag "github.com/spf13/pflag"
)

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
}
