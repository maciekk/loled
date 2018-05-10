// List of Lists (LOL) EDitor
//
// NOTES
// - See https://appliedgo.net/tui/ for review of potential TUIs to use.
// - gocui example of input modes: jroimartin/vimeditor.go
//     https://gist.github.com/jroimartin/1ac98d3da7278fa18866c9cae0af6007

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/nsf/termbox-go"
)

var filename = flag.String("f", "./lol.txt",
	"Filename to use for saving and loading.")
var backupSuffix = flag.String("b", "~",
	"Suffix to append to filename for backups. Use empty string to turn off backups.")

var cmdPrompt = "$ "
var whitespace = " 	\n\r"

// source: http://patorjk.com/software/taag/#p=display&f=Ivrit&t=LoLEd
const logo = `
 _          _     _____    _          ___   ___
| |    ___ | |   | ____|__| | __   __/ _ \ / _ \
| |   / _ \| |   |  _| / _` + "`" + ` | \ \ / / | | | | | |
| |__| (_) | |___| |__| (_| |  \ V /| |_| | |_| |
|_____\___/|_____|_____\__,_|   \_/  \___(_)___/
`

var pfxItem = "- "
var pfxFocusedItem = ">>"
var pfxFocusedMovingItem = "▲▼"
var sfxMore = " ▼"

const (
	PANE_MAIN_MAX_WIDTH = 60
	LABEL_TRASH         = "[[TRASH]]"
	LABEL_DONE          = "[[DONE]]"
)

// The "View" component of MVC framework.
type viewData struct {
	// gui in use
	gui *gocui.Gui

	// current list display
	paneMain *gocui.View

	// information pane
	paneInfo *gocui.View

	// echo area for messages
	paneMessage *gocui.View

	// dialog box (normally hidden)
	paneDialog *gocui.View

	// primary editor
	editorLol *LolEditor
}

////////////////////////////////////////
// Singletons
var ds dataStore
var vd viewData

////////////////////////////////////////
// User Interface functions

func dialog(g *gocui.Gui, title, prefill string, multiline bool) *LineEditor {
	const w = 40
	var h int
	if multiline {
		h = 3
	} else {
		h = 2
	}
	maxX, maxY := g.Size()
	if v, err := g.SetView("dialog", maxX/2-w/2, maxY/2-h/2, maxX/2+w/2, maxY/2-h/2+h); err != nil {
		if err != gocui.ErrUnknownView {
			return nil
		}
		v.Frame = true
		v.Editable = true
		le := LineEditor{}
		le.multiline = multiline
		v.Editor = &le
		v.Title = title
		fmt.Fprintf(v, prefill)
		// TODO: what if prefill is multiline?
		v.SetCursor(len(prefill), 0)
		vd.paneDialog = v
		g.Cursor = true
		if _, err := g.SetCurrentView("dialog"); err != nil {
			panic(err)
		}
		return &le
	}
	return nil
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	var dimsMain, dimsInfo, dimsMsg [4]int
	if maxX < 80 {
		// Vertical layout
		infoPaneHeight := 5
		mainPaneHeight := maxY - infoPaneHeight - 10 - 1
		dimsMain = [4]int{
			0,
			0,
			maxX - 1,
			mainPaneHeight,
		}
		dimsInfo = [4]int{
			0,
			mainPaneHeight + 1,
			maxX - 1,
			mainPaneHeight + infoPaneHeight,
		}
		dimsMsg = [4]int{
			0,
			mainPaneHeight + infoPaneHeight + 1,
			maxX - 1,
			maxY - 1,
		}
	} else {
		// Horizontal layout
		mainPaneWidth := min(PANE_MAIN_MAX_WIDTH, maxX-40)
		dimsMain = [4]int{
			0,
			0,
			mainPaneWidth,
			maxY - 1,
		}
		secondColumnStart := mainPaneWidth + 1
		infoPaneHeight := 5 + 2
		dimsInfo = [4]int{
			secondColumnStart,
			0,
			maxX - 1,
			infoPaneHeight,
		}
		dimsMsg = [4]int{
			secondColumnStart + 3,
			infoPaneHeight + 2,
			maxX - 1,
			maxY - 1,
		}

	}
	if v, err := g.SetView("main", dimsMain[0], dimsMain[1], dimsMain[2], dimsMain[3]); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		vd.editorLol = &LolEditor{}
		v.Editor = vd.editorLol
		v.Editable = true
		v.Highlight = false // to be toggled on once list has items
		// To pick colors from 256, see:
		//   https://en.wikipedia.org/wiki/ANSI_escape_code#8-bit
		// NOTE: gocui has off by 1 error; pick color from above, then
		// add 1.
		v.BgColor = 237 + 1
		v.FgColor = 7 + 1
		v.SelBgColor = 32 + 1
		v.SelFgColor = 231 + 1 | gocui.AttrBold
		vd.paneMain = v
		updateMainPane()
		g.SetCurrentView("main")
		if len(ds.currentList.sublist) > 0 {
			ds.setCurrentItemUsingIndex(0)
		}
	}
	if v, err := g.SetView("info", dimsInfo[0], dimsInfo[1], dimsInfo[2], dimsInfo[3]); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = true
		v.Title = "Status"
		v.Autoscroll = false
		v.FgColor = 240
		vd.paneInfo = v
		updateStatusPane()
	}
	if v, err := g.SetView("message", dimsMsg[0], dimsMsg[1], dimsMsg[2], dimsMsg[3]); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Title = "Log"
		v.Autoscroll = true
		v.FgColor = 240
		vd.paneMessage = v

		fmt.Fprintln(v, logo)
		fmt.Fprintln(v, "")
		fmt.Fprintln(v, "Welcome.")
	}
	return nil
}

func updateMainPane() {
	vd.paneMain.Clear()

	n := ds.currentList

	if n == nil {
		panic("currentList not found!")
	}

	title := fmt.Sprintf("▶ %v", n.label) // TODO: add more info
	fmt.Fprintln(vd.paneMain, title)
	// NOTE: len() needs to count runes, not bytes (because of Unicode
	// multibyte runes).
	fmt.Fprintln(vd.paneMain, strings.Repeat("─", len([]rune(title))))
	for _, kid := range n.sublist {
		pfx := pfxItem
		if kid == ds.currentItem {
			if vd.editorLol.modeMove {
				pfx = pfxFocusedMovingItem
			} else {
				pfx = pfxFocusedItem
			}
		}
		sfx := ""
		if len(kid.sublist) > 0 {
			sfx = sfxMore
		}
		line := pfx + kid.label + sfx
		if kid.tagged {
			line = colorString(line, BG_BLACK, FG_CYAN, "")
		}
		fmt.Fprintln(vd.paneMain, line)
	}
	// For now, if you need to update main view, you likely need to update
	// status as well.
	// TODO: find better location, system.
	updateStatusPane()
}

func updateStatusPane() {
	if vd.paneInfo == nil {
		return
	}

	vd.paneInfo.Clear()

	var s string
	if ds.dirty {
		s = "DIRTY"
	} else {
		s = "NOT dirty"
	}
	fmt.Fprintln(vd.paneInfo, s)
}

////////////////////////////////////////
// main

func keybindings(g *gocui.Gui, ds *dataStore) error {
	// backup keybinding to quit
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		Log(err.Error())
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil {
		Log(err.Error())
	}
	// TODO: set up focus appropriately from start
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, setmain); err != nil {
		Log(err.Error())
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			return termbox.Sync()
		}); err != nil {
		panic(err)
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func setmain(g *gocui.Gui, v *gocui.View) error {
	g.SetCurrentView("main")
	return nil
}

func main() {
	flag.Parse()

	// Set up data.
	if _, err := os.Stat(*filename); err == nil {
		ds.load()
		ds.init()
	} else {
		fmt.Printf("Unable to stat %q; creating empty dataStore instead.\n", *filename)
		ds.init()
	}

	// Set up GUI.
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		Log(err.Error())
	}
	defer g.Close()
	vd.gui = g

	// Does this do anything?
	g.SelBgColor = 237 + 1
	g.SelFgColor = 7 + 1

	g.SetManagerFunc(layout)

	if err := keybindings(g, &ds); err != nil {
		Log(err.Error())
	}

	// Main interaction loop.
	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		Log(err.Error())
	}

	fmt.Printf("Quitting... ")
	if ds.dirty {
		// TODO: better dialog, using gocui
		// FWIW, a yes/no dialog attempt by someone else:
		//   https://aqatl.github.io/trego/2017/03/13/simple-prompt-dialog-in-gocui.html
		fmt.Printf("save first? [y/n] ")
		x := readString()
		if strings.HasPrefix(x, "y") {
			ds.save()
		}
	}
}

// vim: fdm=syntax
