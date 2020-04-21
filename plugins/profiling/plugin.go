package profiling

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

var (
	Plugin = node.NewPlugin("Profiling", node.Enabled, configure, run)
)

const CfgProfilingBindAddress = "profiling.bindAddress"

func init() {
	flag.String(CfgProfilingBindAddress, "localhost:6061", "bind address for the pprof server")
}

func configure(_ *node.Plugin) {}

func run(_ *node.Plugin) {
	go http.ListenAndServe(config.Node.GetString(CfgProfilingBindAddress), nil)
}
