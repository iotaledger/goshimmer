package statusscreen

import "github.com/rivo/tview"

type UILog struct {
	Primitive *tview.Grid
}

func NewUILog() *UILog {
	uiLog := &UILog{
		Primitive: tview.NewGrid(),
	}

	return uiLog
}
