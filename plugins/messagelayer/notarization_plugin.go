package messagelayer

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	// NotarizationPluginName is the name of the notarization plugin.
	NotarizationPluginName = "Notarization"
)

type notarizationDependencies struct {
	dig.In
	Tangle *tangle.Tangle
}

var (
	NotarizationPlugin *node.Plugin
	notarizationDeps   = new(notarizationDependencies)

	notarizationManager *notarization.Manager
)

func init() {
	NotarizationPlugin = node.NewPlugin(NotarizationPluginName, deps, node.Enabled, configureNotarizationPlugin, runNotarizationPlugin)
}

func configureNotarizationPlugin(_ *node.Plugin) {
	notarizationManager = newNotarizationManager()
	notarizationDeps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		notarizationDeps.Tangle.Storage.Message(messageID).Consume(func(m *tangle.Message) {
			notarizationManager.OnMessageConfirmed(m)
		})
	}))
	notarizationDeps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Attach(events.NewClosure(func(transactionID ledgerstate.TransactionID) {
		notarizationDeps.Tangle.LedgerState.Transaction(transactionID).Consume(func(t *ledgerstate.Transaction) {
			notarizationManager.OnTransactionConfirmed(t)
		})
	}))
	notarizationDeps.Tangle.ConfirmationOracle.Events().BranchConfirmed.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		notarizationManager.OnBranchConfirmed(branchID)
	}))
	notarizationDeps.Tangle.LedgerState.BranchDAG.Events.BranchCreated.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		notarizationManager.OnBranchCreated(branchID)
	}))
	notarizationDeps.Tangle.ConfirmationOracle.Events().BranchRejected.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		notarizationManager.OnBranchRejected(branchID)
	}))
}

func runNotarizationPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker("Notarization", func(ctx context.Context) {
		<-ctx.Done()
		notarizationDeps.Tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		NotarizationPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newNotarizationManager() *notarization.Manager {
	return notarization.NewManager(
		notarization.NewEpochManager(),
		notarization.NewEpochCommitmentFactory(),
		notarizationDeps.Tangle,
		notarization.MinCommitableEpochAge(NotarizationParameters.MinEpochCommitableDuration))
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func GetLatestEC() *notarization.EpochCommitment {
	return notarizationManager.GetLatestEC()
}
