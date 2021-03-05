package profiling

import (
	"net/http"
	"runtime"
	"sync"

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
	// plugin is the profiling plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// CfgProfilingBindAddress defines the config flag of the profiling binding address.
const CfgProfilingBindAddress = "profiling.bindAddress"

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func init() {
	flag.String(CfgProfilingBindAddress, "127.0.0.1:6061", "bind address for the pprof server")
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	bindAddr := config.Node().String(CfgProfilingBindAddress)

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
	go http.ListenAndServe(bindAddr, nil)
}
