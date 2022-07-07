package block

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Server *echo.Echo
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("WebAPIBlockEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("blocks/:blockID", GetBlock)
	deps.Server.GET("blocks/:blockID/metadata", GetBlockMetadata)
	deps.Server.POST("blocks/payload", PostPayload)

	deps.Server.GET("blocks/sequences/:sequenceID", GetSequence)
	deps.Server.GET("blocks/sequences/:sequenceID/markerindexbranchidmapping", GetMarkerIndexBranchIDMapping)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetSequence //////////////////////////////////////////////////////////////////////////////////////////////////

// GetSequence is the handler for the /blocks/sequences/:sequenceID endpoint.
func GetSequence(c echo.Context) (err error) {
	sequenceID, err := sequenceIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Booker.MarkersManager.Sequence(sequenceID).Consume(func(sequence *markers.Sequence) {
		blockWithLastMarker := deps.Tangle.Booker.MarkersManager.BlockID(markers.NewMarker(sequenceID, sequence.HighestIndex()))
		err = c.String(http.StatusOK, stringify.Struct("Sequence",
			stringify.StructField("ID", sequence.ID()),
			stringify.StructField("LowestIndex", sequence.LowestIndex()),
			stringify.StructField("HighestIndex", sequence.HighestIndex()),
			stringify.StructField("BlockWithLastMarker", blockWithLastMarker),
		))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Sequence with %s", sequenceID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetMarkerIndexBranchIDMapping ////////////////////////////////////////////////////////////////////////////////

// GetMarkerIndexBranchIDMapping is the handler for the /blocks/sequences/:sequenceID/markerindexbranchidmapping endpoint.
func GetMarkerIndexBranchIDMapping(c echo.Context) (err error) {
	sequenceID, err := sequenceIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Storage.MarkerIndexBranchIDMapping(sequenceID).Consume(func(markerIndexBranchIDMapping *tangle.MarkerIndexBranchIDMapping) {
		err = c.String(http.StatusOK, markerIndexBranchIDMapping.String())
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load MarkerIndexBranchIDMapping of %s", sequenceID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBlock ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetBlock is the handler for the /blocks/:blockID endpoint.
func GetBlock(c echo.Context) (err error) {
	blockID, err := blockIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
		var payloadBytes []byte
		payloadBytes, err = block.Payload().Bytes()

		ecRecord := epoch.NewECRecord(block.EI())
		ecRecord.SetECR(block.ECR())
		ecRecord.SetPrevEC(block.PrevEC())

		err = c.JSON(http.StatusOK, jsonmodels.Block{
			ID:                 block.ID().Base58(),
			StrongParents:      block.ParentsByType(tangle.StrongParentType).Base58(),
			WeakParents:        block.ParentsByType(tangle.WeakParentType).Base58(),
			ShallowLikeParents: block.ParentsByType(tangle.ShallowLikeParentType).Base58(),
			StrongChilds:       deps.Tangle.Utils.ApprovingBlockIDs(block.ID(), tangle.StrongChild).Base58(),
			WeakChilds:         deps.Tangle.Utils.ApprovingBlockIDs(block.ID(), tangle.WeakChild).Base58(),
			ShallowLikeChilds:  deps.Tangle.Utils.ApprovingBlockIDs(block.ID(), tangle.ShallowLikeChild).Base58(),
			IssuerPublicKey:    block.IssuerPublicKey().String(),
			IssuingTime:        block.IssuingTime().Unix(),
			SequenceNumber:     block.SequenceNumber(),
			PayloadType:        block.Payload().Type().String(),
			TransactionID: func() string {
				if block.Payload().Type() == devnetvm.TransactionType {
					return block.Payload().(*devnetvm.Transaction).ID().Base58()
				}

				return ""
			}(),
			EC:                   notarization.EC(ecRecord).Base58(),
			EI:                   uint64(block.EI()),
			ECR:                  block.ECR().Base58(),
			PrevEC:               block.PrevEC().Base58(),
			Payload:              payloadBytes,
			Signature:            block.Signature().String(),
			LatestConfirmedEpoch: uint64(block.LatestConfirmedEpoch()),
		})
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load Block with %s", blockID)))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBlockMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// GetBlockMetadata is the handler for the /blocks/:blockID/metadata endpoint.
func GetBlockMetadata(c echo.Context) (err error) {
	blockID, err := blockIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	if deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *tangle.BlockMetadata) {
		err = c.JSON(http.StatusOK, NewBlockMetadata(blockMetadata))
	}) {
		return
	}

	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(fmt.Errorf("failed to load BlockMetadata with %s", blockID)))
}

// NewBlockMetadata returns BlockMetadata from the given tangle.BlockMetadata.
func NewBlockMetadata(metadata *tangle.BlockMetadata) jsonmodels.BlockMetadata {
	branchIDs, _ := deps.Tangle.Booker.BlockBranchIDs(metadata.ID())

	return jsonmodels.BlockMetadata{
		ID:                    metadata.ID().Base58(),
		ReceivedTime:          metadata.ReceivedTime().Unix(),
		Solid:                 metadata.IsSolid(),
		SolidificationTime:    metadata.SolidificationTime().Unix(),
		StructureDetails:      jsonmodels.NewStructureDetails(metadata.StructureDetails()),
		BranchIDs:             lo.Map(branchIDs.Slice(), utxo.TransactionID.Base58),
		AddedBranchIDs:        lo.Map(metadata.AddedBranchIDs().Slice(), utxo.TransactionID.Base58),
		SubtractedBranchIDs:   lo.Map(metadata.SubtractedBranchIDs().Slice(), utxo.TransactionID.Base58),
		Scheduled:             metadata.Scheduled(),
		ScheduledTime:         metadata.ScheduledTime().Unix(),
		Booked:                metadata.IsBooked(),
		BookedTime:            metadata.BookedTime().Unix(),
		ObjectivelyInvalid:    metadata.IsObjectivelyInvalid(),
		SubjectivelyInvalid:   metadata.IsSubjectivelyInvalid(),
		ConfirmationState:     metadata.ConfirmationState(),
		ConfirmationStateTime: metadata.ConfirmationStateTime().Unix(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PostPayload //////////////////////////////////////////////////////////////////////////////////////////////////

// PostPayload is the handler for the /blocks/payload endpoint.
func PostPayload(c echo.Context) error {
	var request jsonmodels.PostPayloadRequest
	if err := c.Bind(&request); err != nil {
		Plugin.LogInfo(err.Error())
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	parsedPayload, _, err := payload.FromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	blk, err := deps.Tangle.IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewPostPayloadResponse(blk))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region blockIDFromContext /////////////////////////////////////////////////////////////////////////////////////////

// blockIDFromContext determines the BlockID from the blockID parameter in an echo.Context. It expects it to
// either be a base58 encoded string or the builtin alias EmptyBlockID.
func blockIDFromContext(c echo.Context) (blockID tangle.BlockID, err error) {
	switch blockIDString := c.Param("blockID"); blockIDString {
	case "EmptyBlockID":
		blockID = tangle.EmptyBlockID
	default:
		err = blockID.FromBase58(blockIDString)
	}

	return
}

// sequenceIDFromContext determines the sequenceID from the sequenceID parameter in an echo.Context.
func sequenceIDFromContext(c echo.Context) (id markers.SequenceID, err error) {
	sequenceIDInt, err := strconv.Atoi(c.Param("sequenceID"))
	if err != nil {
		return
	}

	return markers.SequenceID(sequenceIDInt), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
