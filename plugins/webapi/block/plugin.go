package block

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/faucet"
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

// region Plugin ///////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// Plugin holds the singleton instance of the plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Server      *echo.Echo
	Retainer    *retainer.Retainer
	BlockIssuer *blockissuer.BlockIssuer
}

func init() {
	Plugin = node.NewPlugin("WebAPIBlockEndpoint", deps, node.Enabled, configure)
}

func configure(_ *node.Plugin) {
	deps.Server.GET("blocks/:blockID", GetBlock)
	deps.Server.GET("blocks/:blockID/metadata", GetBlockMetadata)
	deps.Server.POST("blocks/payload", PostPayload)

	// TODO: add markers to be retained by the retainer
	// deps.Server.GET("blocks/sequences/:sequenceID", GetSequence)
	// deps.Server.GET("blocks/sequences/:sequenceID/markerindexconflictidmapping", GetMarkerIndexConflictIDMapping)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// // region GetSequence //////////////////////////////////////////////////////////////////////////////////////////////////
//
// // GetSequence is the handler for the /blocks/sequences/:sequenceID endpoint.
// func GetSequence(c echo.Context) (err error) {
//	sequenceID, err := sequenceIDFromContext(c)
//	if err != nil {
//		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
//	}
//
//	if deps.Tangle.Booker.MarkersManager.Sequence(sequenceID).Consume(func(sequence *markersold.Sequence) {
//		blockWithLastMarker := deps.Tangle.Booker.MarkersManager.BlockID(markersold.NewMarker(sequenceID, sequence.HighestIndex()))
//		err = c.String(http.StatusOK, stringify.Struct("Sequence",
//			stringify.NewStructField("ID", sequence.ID()),
//			stringify.NewStructField("LowestIndex", sequence.LowestIndex()),
//			stringify.NewStructField("HighestIndex", sequence.HighestIndex()),
//			stringify.NewStructField("BlockWithLastMarker", blockWithLastMarker),
//		))
//	}) {
//		return
//	}
//
//	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load Sequence with %s", sequenceID)))
// }
//
// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// // region GetMarkerIndexConflictIDMapping ////////////////////////////////////////////////////////////////////////////////
//
// // GetMarkerIndexConflictIDMapping is the handler for the /blocks/sequences/:sequenceID/markerindexconflictidmapping endpoint.
// func GetMarkerIndexConflictIDMapping(c echo.Context) (err error) {
//	sequenceID, err := sequenceIDFromContext(c)
//	if err != nil {
//		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
//	}
//
//	if deps.Tangle.Storage.MarkerIndexConflictIDMapping(sequenceID).Consume(func(markerIndexConflictIDMapping *tangleold.MarkerIndexConflictIDMapping) {
//		err = c.String(http.StatusOK, markerIndexConflictIDMapping.String())
//	}) {
//		return
//	}
//
//	return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load MarkerIndexConflictIDMapping of %s", sequenceID)))
// }
//
// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GetBlock ///////////////////////////////////////////////////////////////////////////////////////////////////

// GetBlock is the handler for the /blocks/:blockID endpoint.
func GetBlock(c echo.Context) (err error) {
	blockID, err := blockIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	blockMetadata, exists := deps.Retainer.BlockMetadata(blockID)
	if !exists {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load BlockMetadata with %s", blockID)))
	}

	var payloadBytes []byte
	payloadBytes, err = blockMetadata.M.Block.Payload().Bytes()

	if err != nil {
		return c.JSON(http.StatusInternalServerError, jsonmodels.NewErrorResponse(errors.Wrap(err, "failed to load payload bytes")))
	}

	return c.JSON(http.StatusOK, jsonmodels.Block{
		ID:                   blockMetadata.ID().Base58(),
		StrongParents:        blockMetadata.M.Block.ParentsByType(models.StrongParentType).Base58(),
		WeakParents:          blockMetadata.M.Block.ParentsByType(models.WeakParentType).Base58(),
		ShallowLikeParents:   blockMetadata.M.Block.ParentsByType(models.ShallowLikeParentType).Base58(),
		StrongChildren:       blockMetadata.M.StrongChildren.Base58(),
		WeakChildren:         blockMetadata.M.WeakChildren.Base58(),
		LikedInsteadChildren: blockMetadata.M.LikedInsteadChildren.Base58(),
		IssuerPublicKey:      blockMetadata.M.Block.IssuerPublicKey().String(),
		IssuingTime:          blockMetadata.M.Block.IssuingTime().Unix(),
		SequenceNumber:       blockMetadata.M.Block.SequenceNumber(),
		PayloadType:          blockMetadata.M.Block.Payload().Type().String(),
		TransactionID: func() string {
			if blockMetadata.M.Block.Payload().Type() == devnetvm.TransactionType {
				return blockMetadata.M.Block.Payload().(*devnetvm.Transaction).ID().Base58()
			}
			return ""
		}(),
		CommitmentID:        blockMetadata.M.Block.Commitment().ID().Base58(),
		SlotIndex:           uint64(blockMetadata.M.Block.Commitment().Index()),
		CommitmentRootsID:   blockMetadata.M.Block.Commitment().RootsID().Base58(),
		PrevCommitmentID:    blockMetadata.M.Block.Commitment().PrevID().Base58(),
		Payload:             payloadBytes,
		Signature:           blockMetadata.M.Block.Signature().String(),
		LatestConfirmedSlot: uint64(blockMetadata.M.Block.LatestConfirmedSlot()),
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockMetadata ///////////////////////////////////////////////////////////////////////////////////////////

// GetBlockMetadata is the handler for the /blocks/:blockID/metadata endpoint.
func GetBlockMetadata(c echo.Context) (err error) {
	blockID, err := blockIDFromContext(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	blockMetadata, exists := deps.Retainer.BlockMetadata(blockID)
	if !exists {
		return c.JSON(http.StatusNotFound, jsonmodels.NewErrorResponse(errors.Errorf("failed to load BlockMetadata with %s", blockID)))
	}

	return c.JSON(http.StatusOK, blockMetadata)
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

	parsedPayload, err := payloadFromBytes(request.Payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	blk, err := deps.BlockIssuer.IssuePayload(parsedPayload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, jsonmodels.NewErrorResponse(err))
	}

	return c.JSON(http.StatusOK, jsonmodels.NewPostPayloadResponse(blk))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region blockIDFromContext /////////////////////////////////////////////////////////////////////////////////////////

// blockIDFromContext determines the BlockID from the blockID parameter in an echo.Context. It expects it to
// either be a base58 encoded string or the builtin alias EmptyBlockID.
func blockIDFromContext(c echo.Context) (blockID models.BlockID, err error) {
	switch blockIDString := c.Param("blockID"); blockIDString {
	case "EmptyBlockID":
		blockID = models.EmptyBlockID
	default:
		err = blockID.FromBase58(blockIDString)
	}

	return
}

// sequenceIDFromContext determines the sequenceID from the sequenceID parameter in an echo.Context.
// func sequenceIDFromContext(c echo.Context) (id markers.SequenceID, err error) {
//	sequenceIDInt, err := strconv.Atoi(c.Param("sequenceID"))
//	if err != nil {
//		return
//	}
//
//	return markers.SequenceID(sequenceIDInt), nil
// }

func payloadFromBytes(payloadBytes []byte) (parsedPayload payload.Payload, err error) {
	dptype, _, err := payload.TypeFromBytes(payloadBytes)
	if err != nil {
		return nil, err
	}

	switch dptype {
	case payload.GenericDataPayloadType:
		data := &payload.GenericDataPayload{}
		_, err = data.FromBytes(payloadBytes)
		if err != nil {
			return nil, err
		}
		parsedPayload = data

	case devnetvm.TransactionType:
		tx := &devnetvm.Transaction{}
		err = tx.FromBytes(payloadBytes)
		if err != nil {
			return nil, err
		}
		parsedPayload = tx

	case faucet.RequestType:
		req := &faucet.Payload{}
		_, err = req.FromBytes(payloadBytes)
		if err != nil {
			return nil, err
		}
		parsedPayload = req
	default:
		return nil, errors.New("unknown payload type")
	}

	return parsedPayload, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
