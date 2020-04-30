package profiling

import (
	"net/http"
	// import required to profile
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

var (
	// Plugin is the profiling plugin.
	Plugin = node.NewPlugin("Profiling", node.Enabled, configure, run)
)

// CfgProfilingBindAddress defines the config flag of the profiling binding address.
const CfgProfilingBindAddress = "profiling.bindAddress"

func init() {
	flag.String(CfgProfilingBindAddress, "localhost:6061", "bind address for the pprof server")
}

func configure(_ *node.Plugin) {}

func run(_ *node.Plugin) {
	go http.ListenAndServe(config.Node.GetString(CfgProfilingBindAddress), nil)
}
