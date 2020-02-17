package cli

import (
	"fmt"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// AppVersion version number
	AppVersion = "v0.1.1"
	// AppName app code name
	AppName = "GoShimmer"
)

var PLUGIN = node.NewPlugin("CLI", node.Enabled, configure, run)

func onAddPlugin(name string, status int) {
	AddPluginStatus(node.GetPluginIdentifier(name), status)
}

func init() {
	for name, status := range node.GetPlugins() {
		onAddPlugin(name, status)
	}

	node.Events.AddPlugin.Attach(events.NewClosure(onAddPlugin))

	flag.Usage = printUsage
}

func configure(ctx *node.Plugin) {
	fmt.Printf(`
   _____  ____   _____ _    _ _____ __  __ __  __ ______ _____  
  / ____|/ __ \ / ____| |  | |_   _|  \/  |  \/  |  ____|  __ \ 
 | |  __| |  | | (___ | |__| | | | | \  / | \  / | |__  | |__) |
 | | |_ | |  | |\___ \|  __  | | | | |\/| | |\/| |  __| |  _  / 
 | |__| | |__| |____) | |  | |_| |_| |  | | |  | | |____| | \ \ 
  \_____|\____/|_____/|_|  |_|_____|_|  |_|_|  |_|______|_|  \_\
                             %s                                     
`, AppVersion)
	fmt.Println()

	ctx.Node.Logger.Infof("GoShimmer version %s ...", AppVersion)
	ctx.Node.Logger.Info("Loading plugins ...")
}

func run(ctx *node.Plugin) {
	// do nothing; everything is handled in the configure step
}
