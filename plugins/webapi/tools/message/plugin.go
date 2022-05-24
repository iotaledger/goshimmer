package message

import (
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/configuration"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// PluginName is the name of the web API tools messages endpoint plugin.
const PluginName = "WebAPIToolsMessageEndpoint"

var (
	// Plugin is the plugin instance of the web API tools messages endpoint plugin.
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
	deps   = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle    *tangle.Tangle
	Local     *peer.Local
	Config    *configuration.Configuration
	Server    *echo.Echo
	GossipMgr *gossip.Manager `optional:"true"`
}

const (
	routeDiagnostics = "tools/diagnostic"
	// RouteDiagnosticMessages is the API route for message diagnostics.
	RouteDiagnosticMessages = routeDiagnostics + "/messages"
	// RouteDiagnosticsFirstWeakMessageReferences is the API route for first weak message diagnostics.
	RouteDiagnosticsFirstWeakMessageReferences = RouteDiagnosticMessages + "/firstweakreferences"
	// RouteDiagnosticsMessageRank is the API route for message diagnostics with a rank filter.
	RouteDiagnosticsMessageRank = RouteDiagnosticMessages + "/rank/:rank"
	// RouteDiagnosticsUtxoDag is the API route for Utxo Dag diagnostics.
	RouteDiagnosticsUtxoDag = routeDiagnostics + "/utxodag"
	// RouteDiagnosticsBranches is the API route for branches diagnostics.
	RouteDiagnosticsBranches = routeDiagnostics + "/branches"
	// RouteDiagnosticsTips is the API route for tips diagnostics.
	RouteDiagnosticsTips = routeDiagnostics + "/tips"
)

func configure(_ *node.Plugin) {
	deps.Server.GET("tools/message/pastcone", PastconeHandler)
	deps.Server.GET("tools/message/missing", MissingHandler)
	deps.Server.GET("tools/message/missingavailable", MissingAvailableHandler)
	deps.Server.GET("tools/message/approval", ApprovalHandler)
	deps.Server.GET("tools/message/orphanage", OrphanageHandler)
	deps.Server.POST("tools/message", SendMessage)
	deps.Server.GET(RouteDiagnosticMessages, DiagnosticMessagesHandler)
	deps.Server.GET(RouteDiagnosticsFirstWeakMessageReferences, DiagnosticMessagesOnlyFirstWeakReferencesHandler)
	deps.Server.GET(RouteDiagnosticsMessageRank, DiagnosticMessagesRankHandler)
	deps.Server.GET(RouteDiagnosticsUtxoDag, DiagnosticUTXODAGHandler)
	deps.Server.GET(RouteDiagnosticsBranches, DiagnosticBranchesHandler)
	deps.Server.GET(RouteDiagnosticsTips, TipsDiagnosticHandler)
}
