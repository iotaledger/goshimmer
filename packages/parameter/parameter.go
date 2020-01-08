package parameter

import (
	"github.com/iotaledger/hive.go/parameter"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// flags
	configName    = flag.StringP("config", "c", "config", "Filename of the config file without the file extension")
	configDirPath = flag.StringP("config-dir", "d", ".", "Path to the directory containing the config file")

	// Viper
	NodeConfig = viper.New()
)

// FetchConfig fetches config values from a dir defined via CLI flag --config-dir (or the current working dir if not set).
//
// It automatically reads in a single config file starting with "config" (can be changed via the --config CLI flag)
// and ending with: .json, .toml, .yaml or .yml (in this sequence).
func FetchConfig(printConfig bool, ignoreSettingsAtPrint ...[]string) error {
	err := parameter.LoadConfigFile(NodeConfig, *configDirPath, *configName, true, false)
	if err != nil {
		return err
	}

	if printConfig {
		parameter.PrintConfig(NodeConfig, ignoreSettingsAtPrint...)
	}
	return nil
}

// LoadDefaultConfig only binds the flags, but does not load any config file.
func LoadDefaultConfig(printConfig bool) error {
	// only bind the flags
	flag.Parse()
	err := NodeConfig.BindPFlags(flag.CommandLine)
	if err != nil {
		return err
	}

	if printConfig {
		parameter.PrintConfig(NodeConfig)
	}
	return nil
}
