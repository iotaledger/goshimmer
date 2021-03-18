package ledgerstate

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

// region API endpoints ////////////////////////////////////////////////////////////////////////////////////////////////

// GetOutputEndPoint is the handler for the /ledgerstate/outputs/:outputID endpoint.
func GetOutputEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.Output(outputID).Consume(func(output ledgerstate.Output) {
		err = c.JSON(http.StatusOK, NewOutput(output))
	}) {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("output not present in node")))
	}

	return
}

// GetOutputConsumersEndPoint is the handler for the /ledgerstate/outputs/:outputID/consumers endpoint.
func GetOutputConsumersEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	consumers := make([]*ledgerstate.Consumer, 0)
	if !messagelayer.Tangle().LedgerState.Consumers(outputID).Consume(func(consumer *ledgerstate.Consumer) {
		consumers = append(consumers, consumer)
	}) {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("failed to load consumers")))
	}

	return c.JSON(http.StatusOK, NewConsumers(outputID, consumers))
}

// GetOutputMetadataEndPoint is the handler for the /ledgerstate/outputs/:outputID/metadata endpoint.
func GetOutputMetadataEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	if !messagelayer.Tangle().LedgerState.OutputMetadata(outputID).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
		err = c.JSON(http.StatusOK, NewOutputMetadata(outputMetadata))
	}) {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("output metadata not found")))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output represents a JSON model of a ledgerstate.Output.
type Output struct {
	OutputID OutputID         `json:"outputID"`
	Type     string           `json:"type"`
	Balances []ColoredBalance `json:"balances"`
	Address  string           `json:"address"`
}

// ColoredBalance represents a JSON model of a single colored balance.
type ColoredBalance struct {
	Color   string `json:"color"`
	Balance uint64 `json:"balance"`
}

// NewOutput creates a JSON compatible representation of the output.
func NewOutput(output ledgerstate.Output) Output {
	return Output{
		OutputID: NewOutputID(output.ID()),
		Type:     output.Type().String(),
		Balances: func() []ColoredBalance {
			coloredBalances := make([]ColoredBalance, 0)
			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				coloredBalances = append(coloredBalances, ColoredBalance{Color: color.String(), Balance: balance})
				return true
			})
			return coloredBalances
		}(),
		Address: output.Address().Base58(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumers ////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumers is the JSON model of consumers of the output.
type Consumers struct {
	OutputID              string                 `json:"outputID"`
	ConsumingTransactions []ConsumingTransaction `json:"consumers"`
}

// ConsumingTransaction is the JSON model of a consuming transaction.
type ConsumingTransaction struct {
	ID    string `json:"transactionID"`
	Valid string `json:"valid"`
}

// NewConsumers creates a JSON compatible representation of the consumers of the output.
func NewConsumers(outputID ledgerstate.OutputID, consumers []*ledgerstate.Consumer) Consumers {
	return Consumers{
		OutputID: outputID.Base58(),
		ConsumingTransactions: func() []ConsumingTransaction {
			consumingTransactions := make([]ConsumingTransaction, 0)
			for _, consumer := range consumers {
				consumingTransactions = append(consumingTransactions, ConsumingTransaction{
					ID:    consumer.TransactionID().Base58(),
					Valid: consumer.Valid().String(),
				})
			}

			return consumingTransactions
		}(),
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
func NewOutputMetadata(outputMetadata *ledgerstate.OutputMetadata) OutputMetadata {
	return OutputMetadata{
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
