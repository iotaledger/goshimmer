package node

import "github.com/iotaledger/goshimmer/packages/parameter"

var (
    LOG_LEVEL = parameter.AddInt("NODE/LOG_LEVEL", LOG_LEVEL_INFO, "controls the log types that are shown")
    DISABLE_PLUGINS = parameter.AddString("NODE/DISABLE_PLUGINS", "", "a list of plugins that shall be disabled")
)
