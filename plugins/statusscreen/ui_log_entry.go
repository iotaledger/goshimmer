package statusscreen

import (
    "fmt"
    "github.com/gdamore/tcell"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/rivo/tview"
)

type UILogEntry struct {
    Primitive         *tview.Grid
    TimeContainer     *tview.TextView
    MessageContainer  *tview.TextView
    LogLevelContainer *tview.TextView
}

func NewUILogEntry(message StatusMessage) *UILogEntry {
    logEntry := &UILogEntry{
        Primitive:         tview.NewGrid(),
        TimeContainer:     tview.NewTextView(),
        MessageContainer:  tview.NewTextView(),
        LogLevelContainer: tview.NewTextView(),
    }

    logEntry.TimeContainer.SetBackgroundColor(tcell.ColorWhite)
    logEntry.TimeContainer.SetTextColor(tcell.ColorBlack)
    logEntry.TimeContainer.SetDynamicColors(true)

    logEntry.MessageContainer.SetBackgroundColor(tcell.ColorWhite)
    logEntry.MessageContainer.SetTextColor(tcell.ColorBlack)
    logEntry.MessageContainer.SetDynamicColors(true)

    logEntry.LogLevelContainer.SetBackgroundColor(tcell.ColorWhite)
    logEntry.LogLevelContainer.SetTextColor(tcell.ColorBlack)
    logEntry.LogLevelContainer.SetDynamicColors(true)

    textColor := "black::d"
    switch message.LogLevel {
    case node.LOG_LEVEL_INFO:
        fmt.Fprintf(logEntry.LogLevelContainer, " [black::d][ [blue::d]INFO [black::d]]")
    case node.LOG_LEVEL_SUCCESS:
        fmt.Fprintf(logEntry.LogLevelContainer, " [black::d][  [green::d]OK  [black::d]]")
    case node.LOG_LEVEL_WARNING:
        fmt.Fprintf(logEntry.LogLevelContainer, " [black::d][ [yellow::d]WARN [black::d]]")

        textColor = "yellow::d"
    case node.LOG_LEVEL_FAILURE:
        fmt.Fprintf(logEntry.LogLevelContainer, " [black::d][ [red::d]FAIL [black::d]]")

        textColor = "red::d"
    case node.LOG_LEVEL_DEBUG:
        fmt.Fprintf(logEntry.LogLevelContainer, " [black::d][ [black::b]NOTE [black::d]]")

        textColor = "black::b"
    }

    fmt.Fprintf(logEntry.TimeContainer, "  [black::b]" + message.Time.Format("15:04:05"))
    if message.Source == "Node" {
        fmt.Fprintf(logEntry.MessageContainer, "[" + textColor + "]" + message.Message)
    } else {
        fmt.Fprintf(logEntry.MessageContainer, "[" + textColor + "]" + message.Source + ": " + message.Message)
    }

    logEntry.Primitive.
        SetColumns(11, 0, 11).
        SetRows(1).
        SetBorders(false).
        AddItem(logEntry.TimeContainer, 0, 0, 1, 1, 0, 0, false).
        AddItem(logEntry.MessageContainer, 0, 1, 1, 1, 0, 0, false).
        AddItem(logEntry.LogLevelContainer, 0, 2, 1, 1, 0, 0, false)

    return logEntry
}
