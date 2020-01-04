package statusscreen

import (
	"fmt"
	"math"
	"strconv"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"

	//"strconv"
	"time"

	"github.com/gdamore/tcell"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/rivo/tview"
)

var start = time.Now()

var headerInfos = make([]func() (string, string), 0)

func AddHeaderInfo(generator func() (string, string)) {
	headerInfos = append(headerInfos, generator)
}

type UIHeaderBar struct {
	Primitive     *tview.Grid
	LogoContainer *tview.TextView
	InfoContainer *tview.TextView
}

func NewUIHeaderBar() *UIHeaderBar {
	headerBar := &UIHeaderBar{
		Primitive:     tview.NewGrid(),
		LogoContainer: tview.NewTextView(),
		InfoContainer: tview.NewTextView(),
	}

	headerBar.LogoContainer.
		SetTextAlign(tview.AlignLeft).
		SetTextColor(tcell.ColorWhite).
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorDarkMagenta)

	headerBar.InfoContainer.
		SetTextAlign(tview.AlignRight).
		SetTextColor(tcell.ColorWhite).
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorDarkMagenta)

	headerBar.Primitive.
		SetColumns(17, 0).
		SetRows(0).
		SetBorders(false).
		AddItem(headerBar.LogoContainer, 0, 0, 1, 1, 0, 0, false).
		AddItem(headerBar.InfoContainer, 0, 1, 1, 1, 0, 0, false)

	headerBar.printLogo()
	headerBar.Update()

	return headerBar
}

func (headerBar *UIHeaderBar) Update() {
	duration := time.Since(start)

	headerBar.InfoContainer.Clear()

	fmt.Fprintln(headerBar.InfoContainer)
	fmt.Fprintln(headerBar.InfoContainer, "[::d]COO-LESS IOTA PROTOTYPE  -  [::b]Status: [green::b]SYNCED  ")
	for i := 0; i < 3-len(headerInfos); i++ {
		fmt.Fprintln(headerBar.InfoContainer)
	}

	for _, infoGenerator := range headerInfos {
		fieldName, fieldValue := infoGenerator()
		fmt.Fprintf(headerBar.InfoContainer, "[::b]%v: [::d]%40v  ", fieldName, fieldValue)
		fmt.Fprintln(headerBar.InfoContainer)
	}

	outgoing := "0"
	incoming := "0"
	neighbors := "0"
	total := "0"
	myID := "-"
	if autopeering.Selection != nil {
		outgoing = strconv.Itoa(len(autopeering.Selection.GetOutgoingNeighbors()))
		incoming = strconv.Itoa(len(autopeering.Selection.GetIncomingNeighbors()))
		neighbors = strconv.Itoa(len(autopeering.Selection.GetNeighbors()))
	}
	if autopeering.Discovery != nil {
		total = strconv.Itoa(len(autopeering.Discovery.GetVerifiedPeers()))
	}
	if local.GetInstance() != nil {
		myID = local.GetInstance().ID().String()
	}

	fmt.Fprintf(headerBar.InfoContainer, "[::b]Node ID: [::d]%40v  ", myID)
	fmt.Fprintln(headerBar.InfoContainer)
	fmt.Fprintf(headerBar.InfoContainer, "[::b]Neighbors: [::d]%40v  ", outgoing+" chosen / "+incoming+" accepted / "+neighbors+" total")
	fmt.Fprintln(headerBar.InfoContainer)
	fmt.Fprintf(headerBar.InfoContainer, "[::b]Known Peers: [::d]%40v  ", total+" total")
	fmt.Fprintln(headerBar.InfoContainer)
	fmt.Fprintf(headerBar.InfoContainer, "[::b]Uptime: [::d]")

	padded := false
	if int(duration.Seconds())/(60*60*24) > 0 {
		days := int(duration.Hours()) / 24

		numberLength := int(math.Log10(float64(days))) + 1
		padLength := 31 - numberLength

		fmt.Fprintf(headerBar.InfoContainer, "%*v", padLength, "")

		padded = true

		// d
		fmt.Fprintf(headerBar.InfoContainer, "%02dd ", days)
	}

	if int(duration.Seconds())/(60*60) > 0 {
		if !padded {
			fmt.Fprintf(headerBar.InfoContainer, "%29v", "")
			padded = true
		}
		fmt.Fprintf(headerBar.InfoContainer, "%02dh ", int(duration.Hours())%24)
	}

	if int(duration.Seconds())/60 > 0 {
		if !padded {
			fmt.Fprintf(headerBar.InfoContainer, "%33v", "")
			padded = true
		}
		fmt.Fprintf(headerBar.InfoContainer, "%02dm ", int(duration.Minutes())%60)
	}

	if !padded {
		fmt.Fprintf(headerBar.InfoContainer, "%37v", "")
	}
	fmt.Fprintf(headerBar.InfoContainer, "%02ds  ", int(duration.Seconds())%60)
}

func (headerBar *UIHeaderBar) printLogo() {
	fmt.Fprintln(headerBar.LogoContainer, "")
	fmt.Fprintln(headerBar.LogoContainer, "   SHIMMER 0.0.1")
	fmt.Fprintln(headerBar.LogoContainer, "  ┌──────┬──────┐")
	fmt.Fprintln(headerBar.LogoContainer, "    ───┐ │ ┌───")
	fmt.Fprintln(headerBar.LogoContainer, "     ┐ │ │ │ ┌")
	fmt.Fprintln(headerBar.LogoContainer, "     │ └ │ ┘ │")
	fmt.Fprintln(headerBar.LogoContainer, "     └ ┌ │ ┐ ┘")
	fmt.Fprintln(headerBar.LogoContainer, "       │ │ │")
	fmt.Fprintln(headerBar.LogoContainer, "         ┴")
}
