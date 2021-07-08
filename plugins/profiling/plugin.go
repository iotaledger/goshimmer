package profiling

import (
	"net/http"
	"runtime"
	"sync"

	// import required to profile
	_ "net/http/pprof"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the profiling plugin.
const PluginName = "Profiling"

var (
	// plugin is the profiling plugin.
	plugin *node.Plugin
	once   sync.Once
	log    *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	bindAddr := Parameters.BindAddress

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	log.Infof("%s started, bind-address=%s", PluginName, bindAddr)
	go http.ListenAndServe(bindAddr, nil)
}
