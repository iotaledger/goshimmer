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
func GetOutputEndPoint(c echo.Context) error {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	cachedOutput := messagelayer.Tangle().LedgerState.Output(outputID)
	defer cachedOutput.Release()
	// output object is nil if was not found
	output := cachedOutput.Unwrap()
	if output == nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("output not present in node")))
	}

	return c.JSON(http.StatusOK, NewOutput(output))
}

// GetOutputConsumersEndPoint is the handler for the /ledgerstate/outputs/:outputID/consumers endpoint.
func GetOutputConsumersEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	cachedConsumers := messagelayer.Tangle().LedgerState.Consumers(outputID)
	defer cachedConsumers.Release()

	consumers := cachedConsumers.Unwrap()
	// check all returned and unwrapped objects
	for _, consumer := range consumers {
		if consumer == nil {
			return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("failed to load consumers")))
		}
	}

	return c.JSON(http.StatusOK, NewConsumers(outputID, consumers))
}

// GetOutputMetadataEndPoint is the handler for the /ledgerstate/outputs/:outputID/metadata endpoint.
func GetOutputMetadataEndPoint(c echo.Context) (err error) {
	outputID, err := ledgerstate.OutputIDFromBase58(c.Param("outputID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(err))
	}

	cachedOutputMetadata := messagelayer.Tangle().LedgerState.OutputMetadata(outputID)
	defer cachedOutputMetadata.Release()
	// outputMetadata object is nil if was not found
	outputMetadata := cachedOutputMetadata.Unwrap()
	if outputMetadata == nil {
		return c.JSON(http.StatusBadRequest, NewErrorResponse(xerrors.Errorf("output metadata not present in node")))
	}

	return c.JSON(http.StatusOK, NewOutputMetadata(outputMetadata))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output represents a JSON model of a ledgerstate.Output.
type Output struct {
	ID            string           `json:"id"`
	TransactionID string           `json:"transactionID"`
	OutputIndex   uint16           `json:"outputIndex"`
	Type          string           `json:"type"`
	Balances      []ColoredBalance `json:"balances"`
	Address       string           `json:"address"`
}

// ColoredBalance represents a JSON model of a single colored balance.
type ColoredBalance struct {
	Color   string `json:"color"`
	Balance uint64 `json:"balance"`
}

// NewOutput creates a JSON compatible representation of the output.
func NewOutput(output ledgerstate.Output) Output {
	return Output{
		ID:            output.ID().Base58(),
		TransactionID: output.ID().TransactionID().Base58(),
		OutputIndex:   output.ID().OutputIndex(),
		Type:          output.Type().String(),
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
	ConsumerCount         int                    `json:"consumerCount"`
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
		OutputID:      outputID.Base58(),
		ConsumerCount: len(consumers),
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
