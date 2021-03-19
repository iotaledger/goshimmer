package ledgerstate

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/webapi"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetOutputEndPoint is the handler for the /ledgerstate/outputs/:outputID endpoint.
func GetOutputEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.Output(outputID).Consume(func(output ledgerstate.Output) {
		err = c.JSON(http.StatusOK, NewOutput(output))
	}) {
		return c.JSON(http.StatusNotFound, webapi.NewErrorResponse(xerrors.Errorf("failed to load Output with %s", outputID)))
	}

	return
}

// GetOutputConsumersEndPoint is the handler for the /ledgerstate/outputs/:outputID/consumers endpoint.
func GetOutputConsumersEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	cachedConsumers := messagelayer.Tangle().LedgerState.Consumers(outputID)
	defer cachedConsumers.Release()

	return c.JSON(http.StatusOK, NewOutputConsumers(outputID, cachedConsumers.Unwrap()))
}

// GetOutputMetadataEndPoint is the handler for the /ledgerstate/outputs/:outputID/metadata endpoint.
func GetOutputMetadataEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, webapi.NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.OutputMetadata(outputID).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		err = c.JSON(http.StatusOK, NewOutputMetadata(outputMetadata))
	}) {
		return c.JSON(http.StatusNotFound, webapi.NewErrorResponse(xerrors.Errorf("failed to load OutputMetadata with %s", outputID)))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output represents a JSON model of a ledgerstate.Output.
type Output struct {
	OutputID *OutputID         `json:"outputID,omitempty"`
	Type     string            `json:"type"`
	Balances map[string]uint64 `json:"balances"`
	Address  string            `json:"address"`
}

// NewOutput creates a JSON compatible representation of the output.
func NewOutput(output ledgerstate.Output) *Output {
	return &Output{
		OutputID: NewOutputID(output.ID()),
		Type:     output.Type().String(),
		Balances: func() map[string]uint64 {
			coloredBalances := make(map[string]uint64)
			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				coloredBalances[color.String()] = balance
				return true
			})
			return coloredBalances
		}(),
		Address: output.Address().Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputID /////////////////////////////////////////////////////////////////////////////////////////////////////

// OutputID represents the JSON model of a ledgerstate.OutputID.
type OutputID struct {
	Base58        string `json:"base58"`
	TransactionID string `json:"transactionID"`
	OutputIndex   uint16 `json:"outputIndex"`
}

// NewOutputID returns an OutputID from the given ledgerstate.OutputID.
func NewOutputID(outputID ledgerstate.OutputID) *OutputID {
	if outputID == ledgerstate.EmptyOutputID {
		return nil
	}

	return &OutputID{
		Base58:        outputID.Base58(),
		TransactionID: outputID.TransactionID().Base58(),
		OutputIndex:   outputID.OutputIndex(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputConsumers //////////////////////////////////////////////////////////////////////////////////////////////

// OutputConsumers is the JSON model of a collection of Consumers of an Output.
type OutputConsumers struct {
	OutputID  *OutputID   `json:"outputID"`
	Consumers []*Consumer `json:"consumers"`
}

// NewOutputConsumers creates a JSON compatible representation of the Consumers of the Output.
func NewOutputConsumers(outputID ledgerstate.OutputID, consumers []*ledgerstate.Consumer) *OutputConsumers {
	return &OutputConsumers{
		OutputID: NewOutputID(outputID),
		Consumers: func() []*Consumer {
			consumingTransactions := make([]*Consumer, 0)
			for _, consumer := range consumers {
				if consumer != nil {
					consumingTransactions = append(consumingTransactions, NewConsumer(consumer))
				}
			}

			return consumingTransactions
		}(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the JSON model of a ledgerstate.Consumer.
type Consumer struct {
	TransactionID string `json:"transactionID"`
	Valid         string `json:"valid"`
}

// NewConflict returns a Consumer from the given ledgerstate.Consumer.
func NewConsumer(consumer *ledgerstate.Consumer) *Consumer {
	return &Consumer{
		TransactionID: consumer.TransactionID().Base58(),
		Valid:         consumer.Valid().String(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata is a JSON model of a ledgerstate.OutputMetadata.
type OutputMetadata struct {
	ID                 string `json:"ID"`
	BranchID           string `json:"branchID"`
	Solid              bool   `json:"solid"`
	SolidificationTime int64  `json:"solidificationTime"`
	ConsumerCount      int    `json:"consumerCount"`
	FirstConsumer      string `json:"firstConsumer"`
	Finalized          bool   `json:"finalized"`
}

// NewOutputMetadata creates a JSON compatible representation of the output metadata
func NewOutputMetadata(outputMetadata *ledgerstate.OutputMetadata) *OutputMetadata {
	return &OutputMetadata{
		ID:                 outputMetadata.ID().Base58(),
		BranchID:           outputMetadata.BranchID().Base58(),
		Solid:              outputMetadata.Solid(),
		SolidificationTime: outputMetadata.SolidificationTime().Unix(),
		ConsumerCount:      outputMetadata.ConsumerCount(),
		FirstConsumer:      outputMetadata.FirstConsumer().Base58(),
		Finalized:          outputMetadata.Finalized(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
