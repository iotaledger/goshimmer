package profiling

import (
	"net/http"
	// import required to profile
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

// PluginName is the name of the profiling plugin.
const PluginName = "Profiling"

var (
	// Plugin is the profiling plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log    *logger.Logger
)

// CfgProfilingBindAddress defines the config flag of the profiling binding address.
const CfgProfilingBindAddress = "profiling.bindAddress"

func init() {
	flag.String(CfgProfilingBindAddress, "127.0.0.1:6061", "bind address for the pprof server")
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	bindAddr := config.Node.GetString(CfgProfilingBindAddress)
	log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
	go http.ListenAndServe(bindAddr, nil)
}
