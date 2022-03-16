package message

import (
	"encoding/csv"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo"
)

// TipsDiagnosticHandler runs tips diagnostic over the Tangle.
func TipsDiagnosticHandler(c echo.Context) error {
	return runTipsDiagnostic(c)
}

var tipsDiagnosticTableDescription = DiagnosticMessagesTableDescription

type tipsDiagnosticInfo struct {
	*DiagnosticMessagesInfo
}

func (tdi *tipsDiagnosticInfo) toCSVRow() (row []string) {
	messageRow := tdi.DiagnosticMessagesInfo.toCSVRow()
	row = make([]string, 0, 1+len(messageRow))
	row = append(row, messageRow...)
	return row
}

func runTipsDiagnostic(c echo.Context) (err error) {
	response := c.Response()
	response.Header().Set(echo.HeaderContentType, "text/csv")
	response.WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(response)
	if err := csvWriter.Write(tipsDiagnosticTableDescription); err != nil {
		return errors.Errorf("can't write table description row: %w", err)
	}

	tips := deps.Tangle.TipManager.AllTips()

	if err := buildAndWriteTipsDiagnostic(csvWriter, tips); err != nil {
		return errors.Errorf("%w", err)
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return errors.Errorf("csv writer failed after flush: %w", err)
	}
	return nil
}

func buildAndWriteTipsDiagnostic(w *csv.Writer, tips tangle.MessageIDs) (err error) {
	for tipID := range tips {
		messageInfo := getDiagnosticMessageInfo(tipID)
		tipInfo := tipsDiagnosticInfo{
			DiagnosticMessagesInfo: messageInfo,
		}
		if err := w.Write(tipInfo.toCSVRow()); err != nil {
			return errors.Errorf("failed to write tip diagnostic info row: %w", err)
		}
	}
	return nil
}
