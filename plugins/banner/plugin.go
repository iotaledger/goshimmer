package banner

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the banner plugin.
const PluginName = "Banner"

var (
	// plugin is the plugin instance of the banner plugin.
	plugin *node.Plugin
	once   sync.Once
)

const (
	// AppVersion version number
	AppVersion = "v0.2.0"

	// AppName app code name
	AppName = "GoShimmer"
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Disabled, configure, run)
	})
	return plugin
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

func run(ctx *node.Plugin) {}
