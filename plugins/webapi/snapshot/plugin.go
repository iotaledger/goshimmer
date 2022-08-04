package snapshot

import (
	"net/http"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/core/snapshot"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	snapshotFileName = "snapshot.bin"
)

type dependencies struct {
	dig.In

	Server          *echo.Echo
	Tangle          *tangleold.Tangle
	NotarizationMgr *notarization.Manager
}

var (
	// Plugin holds the singleton instance of the Plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin("Snapshot", deps, node.Disabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("snapshot", DumpCurrentLedger)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DumpCurrentLedger ///////////////////////////////////////////////////////////////////////////////////////////////////

// DumpCurrentLedger dumps a snapshot (all unspent UTXO and all of the access mana) from now.
func DumpCurrentLedger(c echo.Context) (err error) {
	ecRecord, lastConfirmedEpoch, err := deps.Tangle.Options.CommitmentFunc()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}

	// lock the entire ledger in notarization manager until the snapshot is created.
	deps.NotarizationMgr.WriteLockLedger()
	defer deps.NotarizationMgr.WriteUnlockLedger()

	headerPord := headerProducer(ecRecord, lastConfirmedEpoch)
	outputWithMetadataProd := snapshot.NewLedgerUTXOStatesProducer(lastConfirmedEpoch, deps.NotarizationMgr)
	epochDiffsProd := snapshot.NewEpochDiffsProducer(lastConfirmedEpoch, ecRecord.EI(), deps.NotarizationMgr)
	activityLogProd := snapshot.NewActivityLogProducer(deps.Tangle.WeightProvider, deps.NotarizationMgr)

	header, err := snapshot.CreateSnapshot(snapshotFileName, headerPord, outputWithMetadataProd, epochDiffsProd, activityLogProd)
	if err != nil {
		Plugin.LogErrorf("unable to get snapshot bytes %s", err)
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(err))
	}
	Plugin.LogInfo("Snapshot information: ")
	Plugin.LogInfo("     Number of outputs: ", header.OutputWithMetadataCount)
	Plugin.LogInfo("     FullEpochIndex: ", header.FullEpochIndex)
	Plugin.LogInfo("     DiffEpochIndex: ", header.DiffEpochIndex)
	Plugin.LogInfo("     LatestECRecord: ", header.LatestECRecord)

	return c.Attachment(snapshotFileName, snapshotFileName)
}

func headerProducer(ecRecord *epoch.ECRecord, lastConfirmedEpoch epoch.Index) snapshot.HeaderProducerFunc {
	return func() (header *ledger.SnapshotHeader, err error) {
		header = &ledger.SnapshotHeader{
			FullEpochIndex: lastConfirmedEpoch,
			DiffEpochIndex: ecRecord.EI(),
			LatestECRecord: ecRecord,
		}
		return header, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
