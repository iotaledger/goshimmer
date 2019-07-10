package statusscreen

import (
	"time"

	"github.com/gdamore/tcell"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/rivo/tview"
)

var statusMessages = make(map[string]*StatusMessage)
var messageLog = make([]*StatusMessage, 0)

var app *tview.Application

func configure(plugin *node.Plugin) {
	node.DEFAULT_LOGGER.Enabled = false

	plugin.Node.AddLogger(DEFAULT_LOGGER)

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		node.DEFAULT_LOGGER.Enabled = true

		if app != nil {
			app.Stop()
		}
	}))
}

func run(plugin *node.Plugin) {
	newPrimitive := func(text string) *tview.TextView {
		textView := tview.NewTextView()

		textView.
			SetTextAlign(tview.AlignLeft).
			SetText(" " + text)

		return textView
	}

	app = tview.NewApplication()

	headerBar := NewUIHeaderBar()

	content := tview.NewGrid()
	content.SetBackgroundColor(tcell.ColorWhite)
	content.SetColumns(0)

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

	frame := tview.NewFrame(grid).
		SetBorders(1, 1, 0, 0, 2, 2)
	frame.SetBackgroundColor(tcell.ColorDarkGray)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyESC {
			daemon.Shutdown()

			return nil
		}

		return event
	})

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		headerBar.Update()

		rows := make([]int, 1)
		rows[0] = 1
		_, _, _, height := content.GetRect()
		for i := 0; i < len(messageLog) && i < height-2; i++ {
			rows = append(rows, 1)
		}

		content.Clear()
		content.SetRows(rows...)

		blankLine := newPrimitive("")
		blankLine.SetBackgroundColor(tcell.ColorWhite)
		content.AddItem(blankLine, 0, 0, 1, 1, 0, 0, false)

		for i, message := range messageLog[len(messageLog)-len(rows)-1+2:] {
			if i < height-2 {
				content.AddItem(NewUILogEntry(*message).Primitive, i+1, 0, 1, 1, 0, 0, false)
			}
		}

		return false
	})

	daemon.BackgroundWorker("Statusscreen Refresher", func() {
		for {
			select {
			case <-daemon.ShutdownSignal:
				return
			case <-time.After(1 * time.Second):
				app.QueueUpdateDraw(func() {})
			}
		}
	})

	daemon.BackgroundWorker("Statusscreen App", func() {
		if err := app.SetRoot(frame, true).SetFocus(frame).Run(); err != nil {
			panic(err)
		}
	})
}

var PLUGIN = node.NewPlugin("Status Screen", configure, run)
