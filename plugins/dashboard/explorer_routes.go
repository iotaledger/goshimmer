package dashboard

import (
	"fmt"
	"net/http"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/labstack/echo"
	"github.com/mr-tron/base58/base58"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/plugins/chat"
	ledgerstateAPI "github.com/iotaledger/goshimmer/plugins/webapi/ledgerstate"
)

// ExplorerBlock defines the struct of the ExplorerBlock.
type ExplorerBlock struct {
	// ID is the block ID.
	ID string `json:"id"`
	// SolidificationTimestamp is the timestamp of the block.
	SolidificationTimestamp int64 `json:"solidification_timestamp"`
	// The time when this block was issued
	IssuanceTimestamp int64 `json:"issuance_timestamp"`
	// The issuer's sequence number of this block.
	SequenceNumber uint64 `json:"sequence_number"`
	// The public key of the issuer who issued this block.
	IssuerPublicKey string `json:"issuer_public_key"`
	// The shortID of the issuer.
	IssuerShortID string `json:"issuer_short_id"`
	// The signature of the block.
	Signature string `json:"signature"`
	// ParentsByType is the map of parents group by type
	ParentsByType map[string][]string `json:"parentsByType"`
	// StrongChildren are the strong children of the block.
	StrongChildren []string `json:"strongChildren"`
	// WeakChildren are the weak children of the block.
	WeakChildren []string `json:"weakChildren"`
	// ShallowLikeChildren are the shallow like children of the block.
	ShallowLikeChildren []string `json:"shallowLikeChildren"`
	// Solid defines the solid status of the block.
	Solid                 bool               `json:"solid"`
	ConflictIDs           []string           `json:"conflictIDs"`
	AddedConflictIDs      []string           `json:"addedConflictIDs"`
	SubtractedConflictIDs []string           `json:"subtractedConflictIDs"`
	Scheduled             bool               `json:"scheduled"`
	Booked                bool               `json:"booked"`
	ObjectivelyInvalid    bool               `json:"objectivelyInvalid"`
	SubjectivelyInvalid   bool               `json:"subjectivelyInvalid"`
	ConfirmationState     confirmation.State `json:"confirmationState"`
	ConfirmationStateTime int64              `json:"confirmationStateTime"`
	// PayloadType defines the type of the payload.
	PayloadType uint32 `json:"payload_type"`
	// Payload is the content of the payload.
	Payload interface{} `json:"payload"`

	// Structure details
	Rank          uint64 `json:"rank"`
	PastMarkerGap uint64 `json:"pastMarkerGap"`
	IsPastMarker  bool   `json:"isPastMarker"`
	PastMarkers   string `json:"pastMarkers"`

	// Epoch commitment
	EC                   string `json:"ec"`
	EI                   uint64 `json:"ei"`
	ECR                  string `json:"ecr"`
	PrevEC               string `json:"prevEC"`
	LatestConfirmedEpoch uint64 `json:"latestConfirmedEpoch"`
}

func createExplorerBlock(blk *tangleold.Block) *ExplorerBlock {
	blockID := blk.ID()
	cachedBlockMetadata := deps.Tangle.Storage.BlockMetadata(blockID)
	defer cachedBlockMetadata.Release()
	blockMetadata, _ := cachedBlockMetadata.Unwrap()

	conflictIDs, _ := deps.Tangle.Booker.BlockConflictIDs(blockID)

	ecRecord := epoch.NewECRecord(blk.ECRecordEI())
	ecRecord.SetECR(blk.ECR())
	ecRecord.SetPrevEC(blk.PrevEC())

	t := &ExplorerBlock{
		ID:                      blockID.Base58(),
		SolidificationTimestamp: blockMetadata.SolidificationTime().Unix(),
		IssuanceTimestamp:       blk.IssuingTime().Unix(),
		IssuerPublicKey:         blk.IssuerPublicKey().String(),
		IssuerShortID:           identity.NewID(blk.IssuerPublicKey()).String(),
		Signature:               blk.Signature().String(),
		SequenceNumber:          blk.SequenceNumber(),
		ParentsByType:           prepareParentReferences(blk),
		StrongChildren:          deps.Tangle.Utils.ApprovingBlockIDs(blockID, tangleold.StrongChild).Base58(),
		WeakChildren:            deps.Tangle.Utils.ApprovingBlockIDs(blockID, tangleold.WeakChild).Base58(),
		ShallowLikeChildren:     deps.Tangle.Utils.ApprovingBlockIDs(blockID, tangleold.ShallowLikeChild).Base58(),
		Solid:                   blockMetadata.IsSolid(),
		ConflictIDs:             lo.Map(lo.Map(conflictIDs.Slice(), utxo.TransactionID.Bytes), base58.Encode),
		AddedConflictIDs:        lo.Map(lo.Map(blockMetadata.AddedConflictIDs().Slice(), utxo.TransactionID.Bytes), base58.Encode),
		SubtractedConflictIDs:   lo.Map(lo.Map(blockMetadata.SubtractedConflictIDs().Slice(), utxo.TransactionID.Bytes), base58.Encode),
		Scheduled:               blockMetadata.Scheduled(),
		Booked:                  blockMetadata.IsBooked(),
		ObjectivelyInvalid:      blockMetadata.IsObjectivelyInvalid(),
		SubjectivelyInvalid:     blockMetadata.IsSubjectivelyInvalid(),
		ConfirmationState:       blockMetadata.ConfirmationState(),
		ConfirmationStateTime:   blockMetadata.ConfirmationStateTime().Unix(),
		PayloadType:             uint32(blk.Payload().Type()),
		Payload:                 ProcessPayload(blk.Payload()),
		EC:                      ecRecord.ComputeEC().Base58(),
		EI:                      uint64(blk.ECRecordEI()),
		ECR:                     blk.ECR().Base58(),
		PrevEC:                  blk.PrevEC().Base58(),
		LatestConfirmedEpoch:    uint64(blk.LatestConfirmedEpoch()),
	}

	if d := blockMetadata.StructureDetails(); d != nil {
		t.Rank = d.Rank()
		t.PastMarkerGap = d.PastMarkerGap()
		t.IsPastMarker = d.IsPastMarker()
		t.PastMarkers = d.PastMarkers().String()
	}

	return t
}

func prepareParentReferences(blk *tangleold.Block) map[string][]string {
	parentsByType := make(map[string][]string)
	blk.ForEachParent(func(parent tangleold.Parent) {
		if _, ok := parentsByType[parent.Type.String()]; !ok {
			parentsByType[parent.Type.String()] = make([]string, 0)
		}
		parentsByType[parent.Type.String()] = append(parentsByType[parent.Type.String()], parent.ID.Base58())
	})
	return parentsByType
}

// ExplorerAddress defines the struct of the ExplorerAddress.
type ExplorerAddress struct {
	Address         string           `json:"address"`
	ExplorerOutputs []ExplorerOutput `json:"explorerOutputs"`
}

// ExplorerOutput defines the struct of the ExplorerOutput.
type ExplorerOutput struct {
	ID                *jsonmodels.OutputID       `json:"id"`
	Output            *jsonmodels.Output         `json:"output"`
	Metadata          *jsonmodels.OutputMetadata `json:"metadata"`
	TxTimestamp       int                        `json:"txTimestamp"`
	ConfirmationState confirmation.State         `json:"confirmationState"`
}

// SearchResult defines the struct of the SearchResult.
type SearchResult struct {
	// Block is the *ExplorerBlock.
	Block *ExplorerBlock `json:"block"`
	// Address is the *ExplorerAddress.
	Address *ExplorerAddress `json:"address"`
}

func setupExplorerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/block/:id", func(c echo.Context) (err error) {
		var blockID tangleold.BlockID
		err = blockID.FromBase58(c.Param("id"))
		if err != nil {
			return
		}

		t, err := findBlock(blockID)
		if err != nil {
			return
		}

		return c.JSON(http.StatusOK, t)
	})

	routeGroup.GET("/address/:id", func(c echo.Context) error {
		addr, err := findAddress(c.Param("id"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, addr)
	})

	routeGroup.GET("/transaction/:transactionID", ledgerstateAPI.GetTransaction)
	routeGroup.GET("/transaction/:transactionID/metadata", ledgerstateAPI.GetTransactionMetadata)
	routeGroup.GET("/transaction/:transactionID/attachments", ledgerstateAPI.GetTransactionAttachments)
	routeGroup.GET("/output/:outputID", ledgerstateAPI.GetOutput)
	routeGroup.GET("/output/:outputID/metadata", ledgerstateAPI.GetOutputMetadata)
	routeGroup.GET("/output/:outputID/consumers", ledgerstateAPI.GetOutputConsumers)
	routeGroup.GET("/conflict/:conflictID", ledgerstateAPI.GetConflict)
	routeGroup.GET("/conflict/:conflictID/children", ledgerstateAPI.GetConflictChildren)
	routeGroup.GET("/conflict/:conflictID/conflicts", ledgerstateAPI.GetConflictConflicts)
	routeGroup.GET("/conflict/:conflictID/voters", ledgerstateAPI.GetConflictVoters)
	routeGroup.POST("/chat", chat.SendChatBlock)

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := c.Param("search")
		result := &SearchResult{}

		searchInByte, err := base58.Decode(search)
		if err != nil {
			return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
		}

		switch len(searchInByte) {
		case devnetvm.AddressLength:
			addr, err := findAddress(search)
			if err == nil {
				result.Address = addr
			}

		case tangleold.BlockIDLength:
			var blockID tangleold.BlockID
			err = blockID.FromBase58(c.Param("id"))
			if err != nil {
				return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
			}

			blk, err := findBlock(blockID)
			if err == nil {
				result.Block = blk
			}

		default:
			return fmt.Errorf("%w: search ID %s", ErrInvalidParameter, search)
		}

		return c.JSON(http.StatusOK, result)
	})
}

func findBlock(blockID tangleold.BlockID) (explorerBlk *ExplorerBlock, err error) {
	if !deps.Tangle.Storage.Block(blockID).Consume(func(blk *tangleold.Block) {
		explorerBlk = createExplorerBlock(blk)
	}) {
		err = fmt.Errorf("%w: block %s", ErrNotFound, blockID.Base58())
	}

	return
}

func findAddress(strAddress string) (*ExplorerAddress, error) {
	address, err := devnetvm.AddressFromBase58EncodedString(strAddress)
	if err != nil {
		return nil, fmt.Errorf("%w: address %s", ErrNotFound, strAddress)
	}

	outputs := make([]ExplorerOutput, 0)

	// get outputids by address
	// deps.Indexer.CachedOutputsOnAddress(address).Consume(func(output ledgerstate.Output) {
	deps.Indexer.CachedAddressOutputMappings(address).Consume(func(addressOutputMapping *indexer.AddressOutputMapping) {
		var metaData *ledger.OutputMetadata
		var timestamp int64

		// get output metadata + confirmation status from conflict of the output
		deps.Tangle.Ledger.Storage.CachedOutputMetadata(addressOutputMapping.OutputID()).Consume(func(outputMetadata *ledger.OutputMetadata) {
			metaData = outputMetadata
		})

		var txID utxo.TransactionID
		deps.Tangle.Ledger.Storage.CachedOutput(addressOutputMapping.OutputID()).Consume(func(output utxo.Output) {
			if output, ok := output.(devnetvm.Output); ok {
				// get the inclusion state info from the transaction that created this output
				txID = output.ID().TransactionID

				deps.Tangle.Ledger.Storage.CachedTransaction(txID).Consume(func(transaction utxo.Transaction) {
					if tx, ok := transaction.(*devnetvm.Transaction); ok {
						timestamp = tx.Essence().Timestamp().Unix()
					}
				})

				// obtain information about the consumer of the output being considered
				confirmedConsumerID := deps.Tangle.Utils.ConfirmedConsumer(output.ID())

				outputs = append(outputs, ExplorerOutput{
					ID:                jsonmodels.NewOutputID(output.ID()),
					Output:            jsonmodels.NewOutput(output),
					Metadata:          jsonmodels.NewOutputMetadata(metaData, confirmedConsumerID),
					TxTimestamp:       int(timestamp),
					ConfirmationState: metaData.ConfirmationState(),
				})
			}
		})
	})

	if len(outputs) == 0 {
		return nil, fmt.Errorf("%w: address %s", ErrNotFound, strAddress)
	}

	return &ExplorerAddress{
		Address:         strAddress,
		ExplorerOutputs: outputs,
	}, nil
}
