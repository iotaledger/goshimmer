package statusscreen

import (
	"os"

	"github.com/gdamore/tcell"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/rivo/tview"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	log   *logger.Logger
	app   *tview.Application
	frame *tview.Frame
)

func configureTview() {
	headerBar := NewUIHeaderBar()

	content := tview.NewGrid()
	content.SetBackgroundColor(tcell.ColorWhite)
	content.SetColumns(0)
	content.SetBorders(false)
	content.SetOffset(0, 0)
	content.SetGap(0, 0)

	footer := newPrimitive("")
	footer.SetBackgroundColor(tcell.ColorDarkMagenta)
	footer.SetTextColor(tcell.ColorWhite)

	grid := tview.NewGrid().
		SetRows(10, 0, 1).
		SetColumns(0).
		SetBorders(false).
		AddItem(headerBar.Primitive, 0, 0, 1, 1, 0, 0, false).
		AddItem(content, 1, 0, 1, 1, 0, 0, false).
		AddItem(footer, 2, 0, 1, 1, 0, 0, false)

	frame = tview.NewFrame(grid).
		SetBorders(1, 1, 0, 0, 2, 2)
	frame.SetBackgroundColor(tcell.ColorDarkGray)

	app = tview.NewApplication()
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// end the daemon on ctrl+c
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyESC {
			daemon.Shutdown()
			return nil
		}
		return event
	})

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		headerBar.Update()

		rows := make([]int, 2)
		rows[0] = 1
		rows[1] = 1
		_, _, _, height := content.GetRect()
		for i := 0; i < len(logMessages) && i < height-2; i++ {
			rows = append(rows, 1)
		}

		content.Clear()
		content.SetRows(rows...)

		blankLine := newPrimitive("")
		blankLine.SetBackgroundColor(tcell.ColorWhite)
		content.AddItem(blankLine, 0, 0, 1, 1, 0, 0, false)

		logStart := len(logMessages) - (len(rows) - 2)
		if logStart < 0 {
			logStart = 0
		}

		for i, message := range logMessages[logStart:] {
			if i < height-2 {
				content.AddItem(NewUILogEntry(*message).Primitive, i+1, 0, 1, 1, 0, 0, false)
			}
		}

		blankLine = newPrimitive("")
		blankLine.SetBackgroundColor(tcell.ColorWhite)
		content.AddItem(blankLine, height-1, 0, 1, 1, 0, 0, false)

		return false
	})
}

func newPrimitive(text string) *tview.TextView {
	textView := tview.NewTextView()
	textView.SetTextAlign(tview.AlignLeft).SetText(" " + text)

	return textView
}

func isTerminal() bool {
	return terminal.IsTerminal(int(os.Stdin.Fd()))
}
