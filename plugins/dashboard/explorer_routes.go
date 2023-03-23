package dashboard

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/mr-tron/base58/base58"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/app/retainer"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm/indexer"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"

	ledgerstateAPI "github.com/iotaledger/goshimmer/plugins/webapi/ledgerstate"
	slotAPI "github.com/iotaledger/goshimmer/plugins/webapi/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
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
	// ParentsByType is the map of parents groups by type
	ParentsByType map[string][]string `json:"parentsByType"`
	// StrongChildren are the strong children of the block.
	StrongChildren []string `json:"strongChildren"`
	// WeakChildren are the weak children of the block.
	WeakChildren []string `json:"weakChildren"`
	// LikedInsteadChildren are the shallow like children of the block.
	LikedInsteadChildren []string `json:"shallowLikeChildren"`
	// Solid defines the solid status of the block.
	Solid                  bool     `json:"solid"`
	ConflictIDs            []string `json:"conflictIDs"`
	AddedConflictIDs       []string `json:"addedConflictIDs"`
	SubtractedConflictIDs  []string `json:"subtractedConflictIDs"`
	Scheduled              bool     `json:"scheduled"`
	Booked                 bool     `json:"booked"`
	Orphaned               bool     `json:"orphaned"`
	ObjectivelyInvalid     bool     `json:"objectivelyInvalid"`
	SubjectivelyInvalid    bool     `json:"subjectivelyInvalid"`
	Acceptance             bool     `json:"acceptance"`
	AcceptanceTime         int64    `json:"acceptanceTime"`
	Confirmation           bool     `json:"confirmation"`
	ConfirmationTime       int64    `json:"confirmationTime"`
	ConfirmationBySlot     bool     `json:"confirmationBySlot"`
	ConfirmationBySlotTime int64    `json:"confirmationBySlotTime"`
	// PayloadType defines the type of the payload.
	PayloadType payload.Type `json:"payload_type"`
	// Payload is the content of the payload.
	Payload interface{} `json:"payload"`

	// Structure details
	Rank          uint64 `json:"rank"`
	PastMarkerGap uint64 `json:"pastMarkerGap"`
	IsPastMarker  bool   `json:"isPastMarker"`
	PastMarkers   string `json:"pastMarkers"`

	// Slot commitment
	CommitmentID         string `json:"ec"`
	EI                   uint64 `json:"ei"`
	CommitmentRootsID    string `json:"ecr"`
	PreviousCommitmentID string `json:"prevEC"`
	CumulativeWeight     int64  `json:"cumulativeWeight"`
	LatestConfirmedSlot  uint64 `json:"latestConfirmedSlot"`
}

func createExplorerBlock(block *models.Block, blockMetadata *retainer.BlockMetadata) *ExplorerBlock {
	var conflictIDs, addedConflictIDs, subtractedConflictIDs []string
	if blockMetadata.M.ConflictIDs != nil {
		conflictIDs = lo.Map(lo.Map(blockMetadata.M.ConflictIDs.Slice(), packTransactionID), base58.Encode)
	}
	if blockMetadata.M.AddedConflictIDs != nil {
		addedConflictIDs = lo.Map(lo.Map(blockMetadata.M.AddedConflictIDs.Slice(), packTransactionID), base58.Encode)
	}
	if blockMetadata.M.SubtractedConflictIDs != nil {
		subtractedConflictIDs = lo.Map(lo.Map(blockMetadata.M.SubtractedConflictIDs.Slice(), packTransactionID), base58.Encode)
	}
	t := &ExplorerBlock{
		ID:                      block.ID().Base58(),
		SolidificationTimestamp: blockMetadata.M.SolidTime.Unix(),
		IssuanceTimestamp:       block.IssuingTime().Unix(),
		IssuerPublicKey:         block.IssuerPublicKey().String(),
		IssuerShortID:           identity.NewID(block.IssuerPublicKey()).String(),
		Signature:               block.Signature().String(),
		SequenceNumber:          block.SequenceNumber(),
		ParentsByType:           prepareParentReferences(block),
		StrongChildren:          blockMetadata.M.StrongChildren.Base58(),
		WeakChildren:            blockMetadata.M.WeakChildren.Base58(),
		LikedInsteadChildren:    blockMetadata.M.LikedInsteadChildren.Base58(),
		Solid:                   blockMetadata.M.Solid,
		ConflictIDs:             conflictIDs,
		AddedConflictIDs:        addedConflictIDs,
		SubtractedConflictIDs:   subtractedConflictIDs,
		Scheduled:               blockMetadata.M.Scheduled,
		Booked:                  blockMetadata.M.Booked,
		Orphaned:                blockMetadata.M.Orphaned,
		ObjectivelyInvalid:      blockMetadata.M.Invalid,
		SubjectivelyInvalid:     blockMetadata.M.SubjectivelyInvalid,
		Acceptance:              blockMetadata.M.Accepted,
		AcceptanceTime:          blockMetadata.M.AcceptedTime.Unix(),
		Confirmation:            blockMetadata.M.Confirmed,
		ConfirmationTime:        blockMetadata.M.ConfirmedTime.Unix(),
		ConfirmationBySlot:      blockMetadata.M.ConfirmedBySlot,
		ConfirmationBySlotTime:  blockMetadata.M.ConfirmedBySlotTime.Unix(),

		PayloadType:          block.Payload().Type(),
		Payload:              ProcessPayload(block.Payload()),
		CommitmentID:         block.Commitment().ID().Base58(),
		EI:                   uint64(block.Commitment().Index()),
		CommitmentRootsID:    block.Commitment().RootsID().Base58(),
		PreviousCommitmentID: block.Commitment().PrevID().Base58(),
		CumulativeWeight:     block.Commitment().CumulativeWeight(),
		LatestConfirmedSlot:  uint64(block.LatestConfirmedSlot()),
	}

	if d := blockMetadata.M.StructureDetails; d != nil {
		t.Rank = d.Rank
		t.PastMarkerGap = d.PastMarkerGap
		t.IsPastMarker = d.IsPastMarker
		t.PastMarkers = fmt.Sprintf("Markers{%+v}", d.PastMarkers)
	}

	return t
}

func packTransactionID(txID utxo.TransactionID) []byte {
	return lo.PanicOnErr(txID.Bytes())
}

func prepareParentReferences(blk *models.Block) map[string][]string {
	parentsByType := make(map[string][]string)
	blk.ForEachParent(func(parent models.Parent) {
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
		var blockID models.BlockID
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
	routeGroup.GET("/slot/:index", slotAPI.GetCommittedSlot)
	routeGroup.GET("/slot/:index/blocks", slotAPI.GetBlocks)
	routeGroup.GET("/slot/commitment/:commitment", slotAPI.GetCommittedSlotByCommitment)
	routeGroup.GET("/slot/:index/transactions", slotAPI.GetTransactions)
	routeGroup.GET("/slot/:index/utxos", slotAPI.GetUTXOs)

	routeGroup.GET("/search/:search", func(c echo.Context) error {
		search := c.Param("search")
		result := &SearchResult{}

		switch strings.Contains(search, ":") {
		case true:
			var blockID models.BlockID
			err := blockID.FromBase58(search)
			if err != nil {
				return errors.WithMessagef(ErrInvalidParameter, "search ID %s", search)
			}

			blk, err := findBlock(blockID)
			if err != nil {
				return fmt.Errorf("can't find block %s: %w", search, err)
			}
			result.Block = blk

		case false:
			addr, err := findAddress(search)
			if err != nil {
				return fmt.Errorf("can't find address %s: %w", search, err)
			}
			result.Address = addr
		}

		return c.JSON(http.StatusOK, result)
	})
}

func findBlock(blockID models.BlockID) (explorerBlk *ExplorerBlock, err error) {
	blockMetadata, exists := deps.Retainer.BlockMetadata(blockID)
	if !exists {
		return nil, errors.WithMessagef(ErrNotFound, "block metadata %s", blockID.Base58())
	}

	explorerBlk = createExplorerBlock(blockMetadata.M.Block, blockMetadata)

	return
}

func findAddress(strAddress string) (*ExplorerAddress, error) {
	address, err := devnetvm.AddressFromBase58EncodedString(strAddress)
	if err != nil {
		return nil, errors.WithMessagef(ErrNotFound, "address %s", strAddress)
	}

	outputs := make([]ExplorerOutput, 0)

	// get outputids by address
	// deps.Indexer.CachedOutputsOnAddress(address).Consume(func(output ledgerstate.Output) {
	deps.Indexer.CachedAddressOutputMappings(address).Consume(func(addressOutputMapping *indexer.AddressOutputMapping) {
		var metaData *mempool.OutputMetadata
		var timestamp int64

		// get output metadata + confirmation status from conflict of the output
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutputMetadata(addressOutputMapping.OutputID()).Consume(func(outputMetadata *mempool.OutputMetadata) {
			metaData = outputMetadata
		})

		var txID utxo.TransactionID
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedOutput(addressOutputMapping.OutputID()).Consume(func(output utxo.Output) {
			if output, ok := output.(devnetvm.Output); ok {
				// get the inclusion state info from the transaction that created this output
				txID = output.ID().TransactionID

				deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransaction(txID).Consume(func(transaction utxo.Transaction) {
					if tx, ok := transaction.(*devnetvm.Transaction); ok {
						timestamp = tx.Essence().Timestamp().Unix()
					}
				})

				// obtain information about the consumer of the output being considered
				confirmedConsumerID := deps.Protocol.Engine().Ledger.MemPool().Utils().ConfirmedConsumer(output.ID())

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
		return nil, errors.WithMessagef(ErrNotFound, "address %s", strAddress)
	}

	return &ExplorerAddress{
		Address:         strAddress,
		ExplorerOutputs: outputs,
	}, nil
}
