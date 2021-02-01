package banner

import (
	"fmt"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the banner plugin.
const PluginName = "Banner"

var (
	// plugin is the plugin instance of the banner plugin.
	plugin *node.Plugin
	once   sync.Once

	// AppVersion version number
	AppVersion = "v0.3.6"
	// SimplifiedAppVersion is the version number without commit hash
	SimplifiedAppVersion = simplifiedVersion(AppVersion)
)

const (
	// AppName app code name
	AppName = "GoShimmer"
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
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

func simplifiedVersion(version string) string {
	// ignore commit hash
	ver := version
	if strings.Contains(ver, "-") {
		ver = strings.Split(ver, "-")[0]
	}
	// attach a "v" at front
	if !strings.Contains(ver, "v") {
		ver = "v" + ver
	}
	return ver
}
