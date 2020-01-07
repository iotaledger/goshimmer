package statusscreen

import (
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/gdamore/tcell"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/rivo/tview"
	"golang.org/x/crypto/ssh/terminal"
)

var statusMessages = make(map[string]*StatusMessage)
var messageLog = make([]*StatusMessage, 0)
var mutex sync.RWMutex

var app *tview.Application

func configure(plugin *node.Plugin) {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

	// don't write anything to stdout anymore
	// as log messages are now stored and displayed via this plugin
	logger.InjectWriters(ioutil.Discard)

	// store any log message for display
	anyLogMsgClosure := events.NewClosure(func(logLevel logger.LogLevel, prefix string, msg string) {
		storeStatusMessage(logLevel, prefix, msg)
	})
	logger.Events.AnyMsg.Attach(anyLogMsgClosure)

	daemon.BackgroundWorker("UI-Detach", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		logger.InjectWriters(os.Stdout)
		logger.Events.AnyMsg.Detach(anyLogMsgClosure)
		if app != nil {
			app.Stop()
		}
	}, 1)
}

func run(plugin *node.Plugin) {
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

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
		mutex.RLock()
		defer mutex.RUnlock()
		headerBar.Update()

		rows := make([]int, 2)
		rows[0] = 1
		rows[1] = 1
		_, _, _, height := content.GetRect()
		for i := 0; i < len(messageLog) && i < height-2; i++ {
			rows = append(rows, 1)
		}

		content.Clear()
		content.SetRows(rows...)

		blankLine := newPrimitive("")
		blankLine.SetBackgroundColor(tcell.ColorWhite)
		content.AddItem(blankLine, 0, 0, 1, 1, 0, 0, false)

		logStart := len(messageLog) - (len(rows) - 2)
		if logStart < 0 {
			logStart = 0
		}

		for i, message := range messageLog[logStart:] {
			if i < height-2 {
				content.AddItem(NewUILogEntry(*message).Primitive, i+1, 0, 1, 1, 0, 0, false)
			}
		}

		blankLine = newPrimitive("")
		blankLine.SetBackgroundColor(tcell.ColorWhite)
		content.AddItem(blankLine, height-1, 0, 1, 1, 0, 0, false)

		return false
	})

	daemon.BackgroundWorker("Statusscreen Refresher", func(shutdownSignal <-chan struct{}) {
		for {
			select {
			case <-shutdownSignal:
				return
			case <-time.After(1 * time.Second):
				app.QueueUpdateDraw(func() {})
			}
		}
	})

	daemon.BackgroundWorker("Statusscreen App", func(shutdownSignal <-chan struct{}) {
		if err := app.SetRoot(frame, true).SetFocus(frame).Run(); err != nil {
			panic(err)
		}
	})
}

var PLUGIN = node.NewPlugin("Status Screen", node.Enabled, configure, run)
