// List of List (LOL) EDitor
//
// TODO
// - add item right after currentItem, rather than always at end
// - when deleting item, need special handling if its sublist is not empty!
// - need ability to tag multiple items in current list
// - ... then command to push tagged items as sublists under new item within
//   current list
// - want GNU readline capabilities for editing longer text lines (e.g., new
//   item entry)
// - keep these TODOs a *.lol (i.e., dogfood)?
// - start using ncurses, so can do side-by-sides, etc.
// - soon will need to figure out how to handle lists too long for screen
//   height (i.e., scrolling)
// - add 'P'rint command, which prints the whole recursive tree? still needed
//   w/ncurses?
// - explore going back to 'sublist' being []*node, rather than []int of IDs
// - list of recent *.lol files edited should itself be a list under root
// - every save, renumber node IDs to compact them, to remove holes.
// - ASCII-ify "so excited" happy face, use as initial logo?
//   http://1.bp.blogspot.com/_xmWIUzqxRic/TMmpH4J0iKI/AAAAAAAAABY/CLvy4P5AowA/s200/happy-face-770659.png
// - alas, would require width ~50 for good recognition, which might be too large
//
// NOTES
// - See https://appliedgo.net/tui/ for review of potential TUIs to use.
// - gocui example of input modes: jroimartin/vimeditor.go
//     https://gist.github.com/jroimartin/1ac98d3da7278fa18866c9cae0af6007

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
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

var pfxItem = "- " // used in both, disk & screen
var pfxFocusedItem = ">> "

func Warning(s string) {
	fmt.Println("WARNING: " + s)
}

const (
	FG_BLACK = 30 + iota
	FG_RED
	FG_GREEN
	FG_YELLOW
	FG_BLUE
	FG_MAGENTA
	FG_CYAN
	FG_WHITE
)

const (
	BG_BLACK = 40 + iota
	BG_RED
	BG_GREEN
	BG_YELLOW
	BG_BLUE
	BG_MAGENTA
	BG_CYAN
	BG_WHITE
)

// NOTE: these don't seem to work os OS X terminal
const (
	FG_BRIGHT_BLACK = 90 + iota
	FG_BRIGHT_RED
	FG_BRIGHT_GREEN
	FG_BRIGHT_YELLOW
	FG_BRIGHT_BLUE
	FG_BRIGHT_MAGENTA
	FG_BRIGHT_CYAN
	FG_BRIGHT_WHITE
)

// NOTE: these don't seem to work os OS X terminal
const (
	BG_BRIGHT_BLACK = 100 + iota
	BG_BRIGHT_RED
	BG_BRIGHT_GREEN
	BG_BRIGHT_YELLOW
	BG_BRIGHT_BLUE
	BG_BRIGHT_MAGENTA
	BG_BRIGHT_CYAN
	BG_BRIGHT_WHITE
)

// The underlying model for all data in this program, these list of lists of
// lists of ..., is essentially a tree. Here, a node is an element in that
// tree.
type node struct {
	// node identifier #
	id int
	// node content payload
	label string
	// ID of parent node
	parent int
	// list of children IDs
	sublist []int
}

// The "Model" component of MVC framework.
type dataStore struct {
	// Repository of all the nodes, keyed by node ID.
	nodes map[int]*node

	// Next free node ID.
	freeID int

	// Current list and currently selected item in it.
	//
	// NOTE: it is possible the list is empty, and thus does not have a
	// selected item. In that case idCurrentItem is negative to indicate
	// this.
	idCurrentList int
	idCurrentItem int

	// Indicates if data has been modified, and needs to be saved.
	dirty bool
}

// The "View" component of MVC framework.
type viewData struct {
	// current list display
	paneMain *gocui.View

	// echo area
	paneEcho *gocui.View
}

type LolEditor struct {
	modeMove bool
	// TODO: probably all of viewData should be moved here
	// TODO: also, probably current list + current item + tagged should
	// move here too.
}

func (le *LolEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if le.modeMove {
		le.MoveMode(v, key, ch, mod)
	} else {
		le.NormalMode(v, key, ch, mod)
	}
}

func (le *LolEditor) MoveMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	idx := ds.currentItemIndex()
	max_idx := len(ds.currentList().sublist) - 1

	switch {
	case ch == 'q' || key == gocui.KeyEnter:
		fmt.Fprintln(vd.paneEcho, "Switched to MOVE mode.")
		le.modeMove = false
	case ch == 'k':
		if idx > 0 {
			ds.moveItemToIndex(idx - 1)
		}
	case ch == 'K' || ch == '0':
		if idx > 0 {
			ds.moveItemToIndex(0)
		}
	case ch == 'j':
		if idx < max_idx {
			ds.moveItemToIndex(idx + 1)
		}
	case ch == 'J' || ch == 'e' || ch == '-':
		if idx < max_idx {
			ds.moveItemToIndex(max_idx)
		}
	}
	updateMainPane()
}

func (le *LolEditor) NormalMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case ch == 'm':
		fmt.Fprintln(vd.paneEcho, "Switched to NORMAL mode.")
		le.modeMove = true
		// TODO: use actually some of these MoveCursor commands on top of
		// current manipulation of currentItem.
		/*
			case ch == 'j':
				v.MoveCursor(0, 1, false)
			case ch == 'k':
				v.MoveCursor(0, -1, false)
			case ch == 'h':
				v.MoveCursor(-1, 0, false)
			case ch == 'l':
				v.MoveCursor(1, 0, false)
			}
		*/
	case ch == 'j':
		cmdNextItem()
	case ch == 'k':
		cmdPrevItem()
	case ch == 'a':
		cmdAddItems()
	case ch == 'D':
		cmdDeleteItem()
	case ch == 'r':
		cmdReplaceItem()
	case ch == '<' || ch == 'u':
		cmdAscend()
	case ch == '>' || key == gocui.KeyEnter:
		cmdDescend()
	case ch == 'S':
		cmdSaveData()
	case ch == 'L':
		cmdLoadData()
		/*
			case ch == 'q':
				quit()
		*/
	}
}

////////////////////////////////////////
// Singletons
var ds dataStore
var vd viewData

////////////////////////////////////////
// methods
func (ds *dataStore) init() {
	// TODO: init() has uneasy relationship currently with any subsequent
	// load(), which we *should* do next. In particular, forcibly adding
	// "root" node now, and/or dirty marker are questionable.
	ds.dirty = false

	root := node{
		0, // ID
		"root",
		-1, // invalid parent id (i.e., no parent)
		make([]int, 0),
	}
	ds.nodes = make(map[int]*node, 1)
	ds.nodes[0] = &root

	ds.freeID = 1

	ds.idCurrentList = 0  // root
	ds.idCurrentItem = -1 // invalid == no current item
}

func (ds *dataStore) currentList() *node {
	if ds.idCurrentList < 0 {
		return nil
	}

	n, ok := ds.nodes[ds.idCurrentList]

	if !ok {
		return nil
	}

	return n
}

func (ds *dataStore) currentItemIndex() int {
	if ds.idCurrentItem < 0 {
		// Error.
		return -1
	}

	for i, k := range ds.currentList().sublist {
		if k == ds.idCurrentItem {
			return i
		}
	}

	panic("Current item not on current list.")
}

func (ds *dataStore) appendItem(s string) {
	// sanity check
	if _, ok := ds.nodes[ds.freeID]; ok {
		panic("freeID is not free")
	}

	n := node{
		ds.freeID,        // ID
		s,                // payload
		ds.idCurrentList, // parent
		make([]int, 0),   // sublist
	}
	ds.nodes[ds.freeID] = &n
	ds.freeID += 1
	kids := &ds.currentList().sublist
	*kids = append(*kids, n.id)

	// Make the latest node the current one.
	ds.idCurrentItem = n.id

	ds.dirty = true
}

// Replace the current item's label with the provided string.
func (ds *dataStore) replaceItem(s string) {
	if ds.idCurrentItem < 0 {
		// No current item.
		return
	}

	n, ok := ds.nodes[ds.idCurrentItem]

	if !ok {
		return
	}

	n.label = s
}

func (ds *dataStore) deleteItem() {
	if ds.idCurrentItem < 0 {
		// no items
		return
	}

	kids := &ds.currentList().sublist
	i := ds.currentItemIndex()
	*kids = append((*kids)[:i], (*kids)[i+1:]...)
	delete(ds.nodes, ds.idCurrentItem)

	// Make sure index is still valid
	i = min(i, len(*kids)-1)
	ds.idCurrentItem = (*kids)[i]
}

func (ds *dataStore) moveItemToIndex(idxNew int) {
	idx := ds.currentItemIndex()
	if idx == idxNew {
		return
	}

	newSublist := make([]int, 0)
	for _, k := range ds.currentList().sublist {
		if k == ds.idCurrentItem {
			continue
		}
		if len(newSublist) == idxNew {
			newSublist = append(newSublist, ds.idCurrentItem)
		}
		newSublist = append(newSublist, k)
	}
	// Could be being placed as last item.
	if len(newSublist) == idxNew {
		newSublist = append(newSublist, ds.idCurrentItem)
	}
	ds.currentList().sublist = newSublist
	ds.dirty = true
}

func (ds *dataStore) nextItem() {
	list := ds.currentList().sublist
	for i, id := range list {
		if id == ds.idCurrentItem {
			if i < len(list)-1 {
				ds.idCurrentItem = list[i+1]
			}
			return
		}
	}
}

func (ds *dataStore) prevItem() {
	list := ds.currentList().sublist
	for i, id := range list {
		if id == ds.idCurrentItem {
			if i > 0 {
				ds.idCurrentItem = list[i-1]
			}
			return
		}
	}
}

func (ds *dataStore) focusDescend() {
	if ds.idCurrentItem >= 0 {
		ds.idCurrentList = ds.idCurrentItem
		if len(ds.currentList().sublist) > 0 {
			ds.idCurrentItem = ds.currentList().sublist[0]
		} else {
			// < 0 means no item selected
			ds.idCurrentItem = -1
		}
	}
	// Else do nothing; < 0 implies ds.idCurrentList does not have items.
}

func (ds *dataStore) focusAscend() {
	if ds.currentList().parent < 0 {
		// Nothing to do if already at a root.
		return
	}
	ds.idCurrentList, ds.idCurrentItem = ds.currentList().parent, ds.idCurrentList
}

func (ds *dataStore) save() {
	// First, if file exists, attempt to move old version to backup
	// filename.
	if _, err := os.Stat(*filename); err == nil && *backupSuffix != "" {
		exec.Command("cp", "-a", *filename, *filename+*backupSuffix).Run()
	}

	f, err := os.Create(*filename)
	defer f.Close()

	if err != nil {
		fmt.Printf("Error saving to %q: %q\n", filename, err)
		return
	}

	keys := make([]int, 0)
	for _, n := range ds.nodes {
		keys = append(keys, n.id)
	}
	sort.Ints(keys)
	for _, k := range keys {
		n := ds.nodes[k]
		f.WriteString(fmt.Sprintf("node %v\n", k))
		f.WriteString(fmt.Sprintf("%s\n", n.label))
		// TODO: get rid of trailing space after last item; use some
		// join()
		for _, child := range n.sublist {
			f.WriteString(fmt.Sprintf("%v ", child))
		}
		// NOTE: if no children, will result in blank line.
		// (intentional)
		f.WriteString("\n")
	}

	ds.dirty = false
	fmt.Fprintf(vd.paneEcho, "Saved to %q.\n", *filename)
}

func (ds *dataStore) load() {
	// First wipe any data we have.
	ds.nodes = make(map[int]*node, 0)
	ds.freeID = 1

	data, err := ioutil.ReadFile(*filename)
	if err != nil {
		fmt.Printf("Error reading %q: %q\n", filename, err)
		return
	}

	lines := strings.Split(string(data), "\n")

	var l string
	for {
		if len(lines) < 1 {
			break
		}

		l = pop(&lines)

		// Skip any blank lines.
		if strings.Trim(l, whitespace) == "" {
			continue
		}

		if !strings.HasPrefix(l, "node ") {
			fmt.Printf("Format error: expected node #, got %q.\n", l)
			return
		}

		l = l[5:] // Strip "node ".
		id, err := strconv.Atoi(l)

		if err != nil {
			panic(err)
		}

		if ds.freeID <= id {
			ds.freeID = id + 1
		}

		label := pop(&lines)

		l = strings.Trim(pop(&lines), whitespace)
		var idKids []int
		if len(l) > 0 {
			// have some kids
			kids := strings.Split(l, " ")
			idKids = make([]int, len(kids))
			for i, s := range kids {
				idKids[i], err = strconv.Atoi(s)
				if err != nil {
					panic(err)
				}
			}
		} else {
			// no kids
			idKids = make([]int, 0)
		}

		// Create the node.
		n := node{
			id,
			label,
			0, // parent; TBD
			idKids,
		}
		ds.nodes[id] = &n
	}

	// Readjust parent pointers.
	for _, n := range ds.nodes {
		for _, k := range n.sublist {
			ds.nodes[k].parent = n.id
		}
	}
	// Also adjust root's parent.
	ds.nodes[0].parent = -1

	ds.idCurrentList = 0
	ds.idCurrentItem = -1
	if len(ds.nodes[ds.idCurrentList].sublist) > 0 {
		ds.idCurrentItem = ds.nodes[ds.idCurrentList].sublist[0]
	}

	ds.dirty = false
	fmt.Printf("Loaded %q.\n", *filename)
}

////////////////////////////////////////
// User Interface functions

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", 0, 0, min(80, maxX-2), maxY-3-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editor = &LolEditor{}
		v.Editable = true
		vd.paneMain = v
		updateMainPane()
	}
	if v, err := g.SetView("echo", -1, maxY-3, maxX+1, maxY+1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = true
		v.Autoscroll = true
		v.FgColor = gocui.ColorWhite
		vd.paneEcho = v

		fmt.Fprintln(v, "Welcome.")
	}
	g.SetCurrentView("main")
	return nil
}

func updateMainPane() {
	vd.paneMain.Clear()

	n := ds.currentList()

	title := fmt.Sprintf("[[ %v ]]", n.label)
	fmt.Fprintln(vd.paneMain, colorString(title, BG_BLACK, FG_WHITE, ";1"))
	fmt.Fprintln(vd.paneMain, strings.Repeat("=", len(title)))
	for _, item := range n.sublist {
		pfx := pfxItem
		if item == ds.idCurrentItem {
			pfx = pfxFocusedItem
		}
		sfx := ""
		if len(ds.nodes[item].sublist) > 0 {
			sfx = " ..."
		}
		fmt.Fprintln(vd.paneMain, pfx+ds.nodes[item].label+sfx)
	}
}

func readKey() byte {
	// Source: https://stackoverflow.com/questions/15159118/read-a-character-from-standard-input-in-go-without-pressing-enter

	// NOTE: original had "-F", but on OSX it seems to be "-f".

	// disable input buffering
	exec.Command("/bin/stty", "-f", "/dev/tty", "cbreak", "min", "1").Run()
	// do not display entered characters on the screen
	exec.Command("/bin/stty", "-f", "/dev/tty", "-echo").Run()

	defer exec.Command("/bin/stty", "-f", "/dev/tty", "echo").Run()

	var b []byte = make([]byte, 1)
	os.Stdin.Read(b)

	return b[0]
}

func readString() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}
	return text
}

func colorString(s string, fg, bg int, extra string) string {
	return fmt.Sprintf("\033[%d;%d%vm%v\033[0m", bg, fg, extra, s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

////////////////////////////////////////
// COMMANDS

func cmdAddItems() error {
	for {
		fmt.Print("Enter item: ")
		text := strings.TrimRight(readString(), whitespace)
		if text == "" {
			break
		}
		ds.appendItem(text)
	}
	updateMainPane()
	return nil
}

func cmdDeleteItem() error {
	ds.deleteItem()
	updateMainPane()
	return nil
}

func cmdNextItem() error {
	ds.nextItem()
	updateMainPane()
	return nil
}

func cmdPrevItem() error {
	ds.prevItem()
	updateMainPane()
	return nil
}

func cmdDescend() error {
	ds.focusDescend()
	updateMainPane()
	return nil
}

func cmdAscend() error {
	ds.focusAscend()
	updateMainPane()
	return nil
}

func cmdReplaceItem() error {
	fmt.Printf("Replace with: ")
	ds.replaceItem(readString())
	updateMainPane()
	return nil
}

func cmdSaveData() error {
	ds.save()
	return nil
}

func cmdLoadData() error {
	ds.load()
	return nil
}

////////////////////////////////////////
// string manipulation

func pop(l *[]string) string {
	v := (*l)[0]
	*l = (*l)[1:]
	return v
}

func pushBack(l *[]string, s string) {
	*l = append(*l, s)
}

////////////////////////////////////////
// main

func keybindings(g *gocui.Gui, ds *dataStore) error {
	// backup keybinding to quit
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
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

func main() {
	flag.Parse()

	if _, err := os.Stat(*filename); err == nil {
		ds.load()
	} else {
		fmt.Printf("Unable to stat %q; creating empty dataStore instead.\n", *filename)
		ds.init()
	}

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.SetManagerFunc(layout)

	if err := keybindings(g, &ds); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	// TODO: do not use 'fmt', probably won't play nice w/gocui
	fmt.Printf("Quitting... ")
	if ds.dirty {
		fmt.Printf("save first? [y/n] ")
		x := readString()
		if strings.HasPrefix(x, "y") {
			ds.save()
		}
	}
}
