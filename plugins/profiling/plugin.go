package profiling

import (
	"net/http"
	// import required to profile
	_ "net/http/pprof"
	"runtime"

	"github.com/felixge/fgprof"

	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/logger"
)

// PluginName is the name of the profiling plugin.
const PluginName = "Profiling"

var (
	// Plugin is the profiling plugin.
	Plugin *node.Plugin
	log    *logger.Logger
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(_ *node.Plugin) {
	bindAddr := Parameters.BindAddress

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	log.Infof("%s started, bind-address=%s", PluginName, bindAddr)

	http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())

	go http.ListenAndServe(bindAddr, nil)
}
