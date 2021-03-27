package message

import (
	"encoding/csv"
	"net/http"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
	"golang.org/x/xerrors"
)

type tipsDiagnosticType int

const (
	strongTipsOnly tipsDiagnosticType = iota
	weakTipsOnly
	allTips
)

// TipsDiagnosticHandler runs tips diagnostic over the Tangle.
func TipsDiagnosticHandler(c echo.Context) error {
	return runTipsDiagnostic(c, allTips)
}

// StrongTipsDiagnosticHandler runs strong tips diagnostic  over the Tangle.
func StrongTipsDiagnosticHandler(c echo.Context) error {
	return runTipsDiagnostic(c, strongTipsOnly)
}

// WeakTipsDiagnosticHandler runs weak tips diagnostic over the Tangle.
func WeakTipsDiagnosticHandler(c echo.Context) error {
	return runTipsDiagnostic(c, weakTipsOnly)
}

var tipsDiagnosticTableDescription = append([]string{"tipType"}, DiagnosticMessagesTableDescription...)

type tipsDiagnosticInfo struct {
	tipType tangle.TipType
	*DiagnosticMessagesInfo
}

func (tdi *tipsDiagnosticInfo) toCSVRow() (row []string) {
	messageRow := tdi.DiagnosticMessagesInfo.toCSVRow()
	row = make([]string, 0, 1+len(messageRow))
	row = append(row, tdi.tipType.String())
	row = append(row, messageRow...)
	return row
}

func runTipsDiagnostic(c echo.Context, diagnosticType tipsDiagnosticType) (err error) {
	response := c.Response()
	response.Header().Set(echo.HeaderContentType, "text/csv")
	response.WriteHeader(http.StatusOK)

	csvWriter := csv.NewWriter(response)
	if err := csvWriter.Write(tipsDiagnosticTableDescription); err != nil {
		return xerrors.Errorf("can't write table description row: %w", err)
	}
	var strongTips, weakTips tangle.MessageIDs
	if diagnosticType == strongTipsOnly || diagnosticType == allTips {
		strongTips = messagelayer.Tangle().TipManager.AllStrongTips()
	}
	if diagnosticType == weakTipsOnly || diagnosticType == allTips {
		weakTips = messagelayer.Tangle().TipManager.AllWeakTips()
	}
	if err := buildAndWriteTipsDiagnostic(csvWriter, strongTips, tangle.StrongTip); err != nil {
		return xerrors.Errorf("%w", err)
	}
	if err := buildAndWriteTipsDiagnostic(csvWriter, weakTips, tangle.WeakTip); err != nil {
		return xerrors.Errorf("%w", err)
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return xerrors.Errorf("csv writer failed after flush: %w", err)
	}
	return nil
}

func buildAndWriteTipsDiagnostic(w *csv.Writer, tips tangle.MessageIDs, tipType tangle.TipType) (err error) {
	for _, tipID := range tips {
		messageInfo := getDiagnosticMessageInfo(tipID)
		tipInfo := tipsDiagnosticInfo{
			tipType:                tipType,
			DiagnosticMessagesInfo: messageInfo,
		}
		if err := w.Write(tipInfo.toCSVRow()); err != nil {
			return xerrors.Errorf("failed to write tip diagnostic info row: %w", err)
		}
	}
	return nil
}
