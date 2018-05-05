// List of Lists (LOL) EDitor
//
// TODO
// - maybe no node should have ID 0, not even root, to make clear when using
//   uninitialized int? (root could be 1)
// - rather than printing "root", the root node should be labeled with
//   filename being edited.
// - load() should probably reset current List and item to root, first item
// - it should also probably reset Target (some other ops probably as well)
// - clean up finally the singletons (vd & ds), and distribute methods better!
// - now that can hop with random-access, need "go back" command
// - maybe we should have ability to memorize and jump to a memorized location
//   (e.g., Vim's 'm' and apostrophe normal mode commands)
// - visual revamp
//   - better method for printing color strings, w/o ANSII escapes
// - fix gocui off-by-one bug w/256 color setting in SelFgCol and SelBgCol
//   (possibly bug in termbox-go underneath gocui)
// - provide high level overview of major classes and their relationships in
//   top-of-file comment here (above TODO section)
// - delete should delete all tagged items in current list if any are tagged,
//   and only otherwise the current item
// - set up proper Vim folding in this file
// - split up this file into pieces (e.g., main.go, datastore.go, ui.go)
// - when deleting item, need special handling if its sublist is not empty!
// - keep these TODOs a *.lol (i.e., dogfood)?
// - soon will need to figure out how to handle lists too long for screen
//   height (i.e., scrolling)
// - explore going back to 'sublist' being []*node, rather than []int of IDs
// - list of recent *.lol files edited should itself be a list under root
// - every save, renumber node IDs to compact them, to remove holes.
// - ASCII-ify "so excited" happy face, use as initial logo?
//   http://1.bp.blogspot.com/_xmWIUzqxRic/TMmpH4J0iKI/AAAAAAAAABY/CLvy4P5AowA/s200/happy-face-770659.png
// - alas, would require width ~50 for good recognition, which might be too large
// - long term would love undo functionality
// - need indicator ("*") when dirty=true, data needs a save.
// - RPG-like reward system for tasks complete
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
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode"

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
)

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
	// Is it tagged?
	tagged bool
}

type Target struct {
	list  int
	index int
}

// The "Model" component of MVC framework.
type dataStore struct {
	// Repository of all the nodes, keyed by node ID.
	nodes map[int]*node

	// Next free node ID for use as key in 'nodes'.
	freeID int

	// Current list and currently selected item in it.
	//
	// NOTE: it is possible the list is empty, and thus does not have a
	// selected item. In that case idCurrentItem is negative to indicate
	// this.
	// TODO: this too should be a Target (and probably name 'cursor').
	idCurrentList int
	idCurrentItem int

	// Indicates if data has been modified, and needs to be saved.
	dirty bool

	// User-defined targets.
	//
	// NOTE: in future we will have multiple; these will be like  "marks"
	// in Vim.
	Mark Target

	// Pre-defined special targets.
	// NOTE: using * so that able to differentiate uninitialized Target.
	Trash *Target // Where deleted items are moved.
}

// The "View" component of MVC framework.
type viewData struct {
	// gui in use
	gui *gocui.Gui

	// current list display
	paneMain *gocui.View

	// echo area for messages
	paneMessage *gocui.View

	// dialog box (normally hidden)
	paneDialog *gocui.View

	// primary editor
	editorLol *LolEditor
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
	new_idx := -1

	switch {
	case ch == 'q' || key == gocui.KeyEnter:
		Log("Switched to NORMAL mode.")
		le.modeMove = false
		updateMainPane()
	case ch == 'k':
		if idx > 0 {
			new_idx = idx - 1
		}
	case ch == 'K' || ch == '0':
		if idx > 0 {
			new_idx = 0
		}
	case ch == 'j':
		if idx < max_idx {
			new_idx = idx + 1
		}
	case ch == 'J' || ch == 'e' || ch == '-':
		if idx < max_idx {
			new_idx = max_idx
		}
	}
	if new_idx >= 0 {
		ds.moveItemToIndex(new_idx)
		updateMainPane()
		// Update the cursor as well.
		ds.setCurrentItemIndex(new_idx)
	}
}

func (le *LolEditor) NormalMode(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case ch == 'm':
		Log("Switched to MOVE mode.")
		le.modeMove = true
		updateMainPane()
	case ch == 'j' || key == gocui.KeyArrowDown:
		cmdNextItem()
	case ch == 'k' || key == gocui.KeyArrowUp:
		cmdPrevItem()
	case ch == 'J' || ch == '$' || ch == '-':
		cmdLastItem()
	case ch == 'K' || ch == '0':
		cmdFirstItem()
	case ch == 'a' || ch == 'o':
		cmdAddItems()
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
	case key == gocui.KeySpace:
		cmdToggleItem()
	case key == gocui.KeyCtrlT:
		// Can't use gocuy.KeyCtrlSpace as it == 0 / matches most
		// regular keypresses (because key == 0 then)
		cmdToggleAllItems()
	case ch == 'g':
		cmdGroupItems()
	case ch == 'G':
		cmdUngroupItems()
	case ch == 't':
		cmdSetUserTarget()
	case ch == 'T':
		cmdGoToUserTarget()
	case ch == 'M':
		cmdMoveToTarget(&ds.Mark)
	case ch == 'D':
		//cmdDeleteItem()
		cmdMoveToTarget(ds.Trash)
		/*
			case ch == 'q':
				quit()
		*/
	default:
		fmt.Printf("\007")
	}
}

type dialogCallback func([]string)

type LineEditor struct {
	multiline bool
	onFinish  dialogCallback
}

// A beefed up version of 'simpleEditor' that resembles Emacs-like bindings
// that are on by default with GNU readline, shells, etc.
func fullerEditor(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	x, y := v.Cursor()
	switch {
	case key == gocui.KeyCtrlB,
		key == gocui.KeyArrowLeft:
		v.MoveCursor(-1, 0, false)
	case key == gocui.KeyCtrlF,
		key == gocui.KeyArrowRight:
		v.MoveCursor(+1, 0, false)
	// None of the following are aware that the line may be scrolled
	// (i.e., origin has moved) TODO: fix
	case key == gocui.KeyCtrlE,
		key == gocui.KeyEnd:
		// end of line
		curLine, err := v.Line(y)
		if err != nil {
			panic(err)
		}
		v.MoveCursor(len(curLine)-x, 0, false)
	case key == gocui.KeyCtrlA,
		key == gocui.KeyHome:
		// beginning of line
		v.MoveCursor(-x, 0, false)
	case key == gocui.KeyCtrlD:
		v.EditDelete(false)
	case key == gocui.KeyCtrlU:
		// erase to beginning of line
		for x > 0 {
			v.EditDelete(true)
			x, y = v.Cursor()
		}
	case key == gocui.KeyCtrlW:
		s, err := v.Line(y)
		if err != nil {
			panic(err)
		}
		if x-1 >= len(s) {
			break
		}
		// Consume backwards all non-whitespace (i.e., last word)
		for x > 0 && !unicode.IsSpace(rune(s[x-1])) {
			x -= 1
			v.EditDelete(true)
		}
		// Now consume the trailing whitespace.
		for x > 0 && unicode.IsSpace(rune(s[x-1])) {
			x -= 1
			v.EditDelete(true)
		}
	default:
		// If we didn't handle key above, pass through to base editor.
		gocui.DefaultEditor.Edit(v, key, ch, mod)
	}
}

func (le *LineEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	onDone := func() {
		le.onFinish(v.BufferLines())
		vd.gui.Cursor = false
		vd.gui.DeleteView("dialog")
		vd.gui.SetCurrentView("main")
	}

	switch {
	case key == gocui.KeyEnter && !le.multiline:
		onDone()
	case key == gocui.KeyEnter && le.multiline:
		lines := v.BufferLines()
		if len(lines) < 1 {
			// User hit Enter, without any text entered; nothing to
			// do.
			onDone()
			return
		}

		// If last line is blank, then that is the signal that we are
		// don with the multi-line entry.
		if strings.Trim(lines[len(lines)-1], whitespace) == "" {
			onDone()
			return
		}
		// Else the Enter was simply advancing to next line, and we
		// continue to edit (let the fullEditor process the Enter as
		// usual).
		fallthrough
	default:
		fullerEditor(v, key, ch, mod)
	}
}

////////////////////////////////////////
// Singletons
var ds dataStore
var vd viewData

func Log(s string, a ...interface{}) {
	var f io.Writer
	if vd.paneMessage != nil {
		f = vd.paneMessage
	} else {
		f = os.Stdout
	}
	fmt.Fprintf(f, s+"\n", a...)
}

////////////////////////////////////////
// methods

// (Finish) initializing data store.
// Has two use-cases:
// - fresh after startup: initializes everything (e.g., incl. 'nodes')
// - after a file load: primarily initializes remaining view parameters (e.g.,
//   cursor)
func (ds *dataStore) init() {
	ds.dirty = false

	// This runs only on startup; 'load' will have populated this.
	if ds.nodes == nil {
		root := node{
			0, // ID
			"root",
			-1, // invalid parent id (i.e., no parent)
			make([]int, 0),
			false, // tagged
		}
		ds.nodes = make(map[int]*node, 1)
		ds.nodes[0] = &root

		ds.freeID = 1
	}

	rootkids := ds.nodes[0].sublist

	// Ensure Trash exists.
	if ds.Trash == nil {
		// First, need cursor at end of root list.
		ds.idCurrentList = 0 // root
		if len(rootkids) > 0 {
			ds.idCurrentItem = rootkids[len(rootkids)-1]
		} else {
			ds.idCurrentItem = -1
		}
		n := ds.appendItem("[Trash]") // TODO: should we NOT set ds.dirty?
		ds.Trash = &Target{n.id, -1}  // -1 id because Trash list empty
	}

	// Reset cursor.
	ds.idCurrentList = 0 // root; TODO: find this by name, in case load() led to diff idx
	if len(rootkids) > 0 {
		ds.idCurrentItem = 0
	} else {
		// invalid == no current item
		ds.idCurrentItem = -1
	}
}

func (ds *dataStore) currentList() *node {
	n, ok := ds.nodes[ds.idCurrentList]
	if !ok {
		return nil
	}
	return n
}

func (ds *dataStore) currentItem() *node {
	n, ok := ds.nodes[ds.idCurrentItem]
	if !ok {
		return nil
	}
	return n
}

// Sets current item and updates the UI selection bar to it.
func (ds *dataStore) setCurrentItemIndex(idx int) {
	items := ds.currentItems()
	if idx < 0 || len(*items) == 0 {
		// no selected item
		ds.idCurrentItem = -1
		if vd.paneMain != nil {
			vd.paneMain.Highlight = false
		}
		return
	}
	if idx > len(*items)-1 {
		panic(fmt.Sprintf(
			"Bad index for setCurrentItemIndex(): %v (len = %v)\n",
			idx, len(*items)))
		if vd.paneMain != nil {
			vd.paneMain.Highlight = false
		}
		return
	}

	ds.idCurrentItem = (*ds.currentItems())[idx]
	if vd.paneMain != nil {
		// +2 offset due to list title and underline.
		vd.paneMain.SetCursor(0, idx+2)
		vd.paneMain.Highlight = true
	}
}

func (ds *dataStore) currentItems() *[]int {
	return &ds.currentList().sublist
}

func (ds *dataStore) indexOfItem(id int) int {
	l := ds.currentList()
	if l == nil {
		Log("indexOfItem(%v) called but no current list.", id)
		return -1
	}
	if l.sublist == nil {
		Log("indexOfItem(%v) called but current list has no sublist.", id)
		return -1
	}
	for i, k := range ds.currentList().sublist {
		if k == id {
			return i
		}
	}
	// Didn't find it.
	return -1
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

func insertKid(newkid int, kids *[]int, pos int) {
	// Sigh, Go has gross item insertion.

	*kids = append(*kids, -1) // extend length by 1
	copy((*kids)[pos+1:], (*kids)[pos:])
	(*kids)[pos] = newkid
}

// Returns the created node.
func (ds *dataStore) appendItem(s string) *node {
	// sanity check
	if _, ok := ds.nodes[ds.freeID]; ok {
		panic("freeID is not free")
	}

	n := node{
		ds.freeID,        // ID
		s,                // payload
		ds.idCurrentList, // parent
		make([]int, 0),   // sublist
		false,            // tagged
	}
	ds.nodes[ds.freeID] = &n
	ds.freeID += 1
	kids := ds.currentItems()
	i := ds.currentItemIndex()
	insertKid(n.id, kids, i+1)

	// Make the latest node the current one.
	ds.setCurrentItemIndex(i + 1)

	ds.dirty = true

	return &n
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

	ds.dirty = true
}

func (ds *dataStore) deleteItem() {
	if ds.idCurrentItem < 0 {
		// no items
		return
	}

	if len(ds.nodes[ds.idCurrentItem].sublist) > 0 {
		// TODO: delete all child nodes too
		Log("Deletion of non-leaf nodes not supported yet.")
		return
	}

	kids := &ds.currentList().sublist
	i := ds.currentItemIndex()

	*kids = append((*kids)[:i], (*kids)[i+1:]...)
	delete(ds.nodes, ds.idCurrentItem)

	// Make sure index is still valid
	i = min(i, len(*kids)-1)
	ds.setCurrentItemIndex(i)

	ds.dirty = true
}

func (ds *dataStore) toggleItem() {
	if ds.idCurrentItem < 0 {
		// no items
		return
	}

	ds.currentItem().tagged = !ds.currentItem().tagged
}

func (ds *dataStore) toggleAllItems() {
	if ds.idCurrentItem < 0 {
		// no items
		return
	}

	newval := !ds.currentItem().tagged
	for _, id := range *ds.currentItems() {
		ds.nodes[id].tagged = newval
	}

}

func (ds *dataStore) SetUserTarget() {
	ds.Mark.list = ds.idCurrentList
	ds.Mark.index = ds.currentItemIndex()
	Log("Target set.")
}

func (ds *dataStore) GoToUserTarget() {
	// TODO: how does this behave when both Target vars are zero? (i.e.,
	// at startup) ID==0 happens to amount to the root list (good), but
	// current item is the root list as well... should trigger "item not
	// in list" warning.
	ds.idCurrentList = ds.Mark.list
	// Check if Target is on a list with no items.
	if ds.Mark.index == -1 {
		ds.idCurrentItem = -1
	} else {
		ds.idCurrentItem = (*ds.currentItems())[ds.Mark.index]
	}
	ds.setCurrentItemIndex(ds.Mark.index)
	Log("Jumped to Target.")
}

// Move cursor to given target 't'.
// Also, if move did occur, advances the target to point at moved item.
func (ds *dataStore) MoveToTarget(t *Target) {
	// Check that there is anything to do.
	if ds.idCurrentItem < 0 || ds.idCurrentList < 0 {
		Log("No current list or item.")
		return
	}

	// First, remove item from current list.
	kids := &ds.currentList().sublist
	i := ds.currentItemIndex()
	*kids = append((*kids)[:i], (*kids)[i+1:]...)

	// Find successor, if any.
	var idNewCurrentItem int
	if i >= len(*kids) {
		i = len(*kids) - 1
	}
	if i < 0 {
		// No more items left on list.
		idNewCurrentItem = -1
	} else {
		idNewCurrentItem = (*kids)[i]
	}

	// Now place it at Target
	kids = &ds.nodes[t.list].sublist
	if len(*kids) == 0 {
		*kids = []int{ds.idCurrentItem}
	} else {
		*kids = append(*kids, -1) // extend length by 1
		i = t.index + 1           // insertion desired AFTER Mark
		// There is a second part to shift only if item to insert is
		// not meant as last item.
		if i < len(*kids)-1 {
			copy((*kids)[i+1:], (*kids)[i:])
		}
		(*kids)[i] = ds.idCurrentItem
	}

	// Next, advance Mark location offset to point to the currently added
	// item, so that NEXT item is added AFTER the one we just added (so
	// that sequence of items added remains in the correct order).
	t.index += 1

	// Finally make sure current item is its former successor.
	ds.idCurrentItem = idNewCurrentItem
}

func (ds *dataStore) ungroupItems() {
	if ds.idCurrentItem < 0 || len(ds.currentItem().sublist) < 1 {
		Log("Cannot ungroup, item invalid or has no sublist.")
		return
	}

	kids := ds.currentItems()
	subkids := ds.currentItem().sublist

	// Remove group node from kids.
	i := ds.currentItemIndex()
	*kids = append((*kids)[:i], (*kids)[i+1:]...)
	delete(ds.nodes, ds.idCurrentItem)

	// Add in the subkids at same index.
	// Iterate in reverse so that end ordering stays unchanged.
	for j := len(subkids) - 1; j >= 0; j-- {
		insertKid(subkids[j], kids, i)
	}

	// Update current item.
	ds.setCurrentItemIndex(i)

	ds.dirty = true
}

func (ds *dataStore) groupTaggedItemsUnder(name string) {
	kids := ds.currentItems()
	listTagged := []int{}
	listUntagged := []int{}
	idxFirstTagged := -1 // not set
	for i, k := range *kids {
		if ds.nodes[k].tagged {
			// TODO: do we really want to clear the tag bit?
			ds.nodes[k].tagged = false
			if len(listTagged) == 0 {
				idxFirstTagged = i
			}
			listTagged = append(listTagged, k)
		} else {
			listUntagged = append(listUntagged, k)
		}
	}

	// Clean up current list.
	*kids = listUntagged

	// Create new node for group.
	n := node{
		ds.freeID,        // ID
		name,             // payload
		ds.idCurrentList, // parent
		listTagged,       // sublist
		false,            // tagged
	}
	ds.nodes[ds.freeID] = &n
	ds.freeID += 1

	// Insert the new node into current list.
	i := idxFirstTagged
	if i > len(*kids) {
		i = len(*kids)
	}
	insertKid(n.id, kids, i)

	// Adjust current item.
	ds.idCurrentItem = n.id

	ds.dirty = true
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
				ds.setCurrentItemIndex(i + 1)
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
				ds.setCurrentItemIndex(i - 1)
			}
			return
		}
	}
}

func (ds *dataStore) firstItem() {
	ds.setCurrentItemIndex(0)
}

func (ds *dataStore) lastItem() {
	l := ds.currentItems()
	ds.setCurrentItemIndex(len(*l) - 1)
}

func (ds *dataStore) focusDescend() {
	if ds.idCurrentItem >= 0 {
		ds.idCurrentList = ds.idCurrentItem
		if len(ds.currentList().sublist) > 0 {
			ds.setCurrentItemIndex(0)
		} else {
			// < 0 means no item selected
			ds.setCurrentItemIndex(-1)
		}
		// TODO: push this off to cmd*()
		//vd.paneMain.Title = ds.currentList().label
	}
	// Else do nothing; < 0 implies ds.idCurrentList does not have items.
}

func (ds *dataStore) focusAscend() {
	if ds.currentList().parent < 0 {
		// Nothing to do if already at a root.
		return
	}
	newCurrentItem := ds.idCurrentList
	ds.idCurrentList = ds.currentList().parent
	ds.setCurrentItemIndex(ds.indexOfItem(newCurrentItem))

	// TODO: push this off to cmd*()
	//vd.paneMain.Title = ds.currentList().label
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
	Log("Saved to %q.", *filename)
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
			false, // tagged
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
	ds.setCurrentItemIndex(-1)
	if len(ds.nodes[ds.idCurrentList].sublist) > 0 {
		ds.setCurrentItemIndex(0)
	}

	ds.dirty = false
	Log("Loaded %q.", *filename)
}

////////////////////////////////////////
// User Interface functions

func dialog(g *gocui.Gui, title, prefill string) *LineEditor {
	const w = 40
	const h = 3
	maxX, maxY := g.Size()
	if v, err := g.SetView("dialog", maxX/2-w/2, maxY/2-h/2, maxX/2+w/2, maxY/2+h/2); err != nil {
		if err != gocui.ErrUnknownView {
			return nil
		}
		v.Frame = true
		v.Editable = true
		le := LineEditor{}
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
	var dimsMain, dimsMsg [4]int
	if maxX < 80 {
		// Vertical layout
		mainPaneHeight := maxY - 10 - 1
		dimsMain = [4]int{
			0,
			0,
			maxX - 1,
			mainPaneHeight,
		}
		dimsMsg = [4]int{
			0,
			mainPaneHeight + 1,
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
		dimsMsg = [4]int{
			mainPaneWidth + 1 + 3,
			0,
			maxX - 1,
			maxY / 2,
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
		if len(*ds.currentItems()) > 0 {
			ds.setCurrentItemIndex(0)
		}
	}
	if v, err := g.SetView("message", dimsMsg[0], dimsMsg[1], dimsMsg[2], dimsMsg[3]); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Title = "log"
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

	n := ds.currentList()

	if n == nil {
		panic("currentList not found!")
	}

	title := fmt.Sprintf("▶ %v", n.label) // TODO: add more info
	fmt.Fprintln(vd.paneMain, title)
	//fmt.Fprintln(vd.paneMain, colorString(title, BG_BLACK, FG_WHITE, ";1"))
	// NOTE: len() needs to count runes, not bytes (because of Unicode
	// multibyte runes).
	fmt.Fprintln(vd.paneMain, strings.Repeat("─", len([]rune(title))))
	for _, item := range n.sublist {
		ni := ds.nodes[item]
		pfx := pfxItem
		if item == ds.idCurrentItem {
			if vd.editorLol.modeMove {
				pfx = pfxFocusedMovingItem
			} else {
				pfx = pfxFocusedItem
			}
		}
		sfx := ""
		if len(ds.nodes[item].sublist) > 0 {
			sfx = sfxMore
		}
		line := pfx + ds.nodes[item].label + sfx
		if ni.tagged {
			line = colorString(line, BG_BLACK, FG_CYAN, "")
		}
		fmt.Fprintln(vd.paneMain, line)
	}
}

func readString() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\r')
	if text[len(text)-1] == '\n' || text[len(text)-1] == '\r' {
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

func cmdAddItems() {
	dlgEditor := dialog(vd.gui, "Add", "")
	dlgEditor.multiline = true
	dlgEditor.onFinish = func(ss []string) {
		for _, s := range ss {
			text := strings.TrimRight(s, whitespace)
			if len(text) > 0 {
				ds.appendItem(text)
			}
		}
		updateMainPane()
	}
}

func cmdReplaceItem() {
	dlgEditor := dialog(vd.gui, "Replace", ds.nodes[ds.idCurrentItem].label)
	dlgEditor.multiline = false
	dlgEditor.onFinish = func(ss []string) {
		ds.replaceItem(ss[0])
		updateMainPane()
	}
}

func cmdDeleteItem() {
	ds.deleteItem()
	updateMainPane()
}

func cmdToggleItem() {
	ds.toggleItem()
	cmdNextItem()
	updateMainPane()
}

func cmdToggleAllItems() {
	ds.toggleAllItems()
	updateMainPane()
}

func cmdGroupItems() {
	dlgEditor := dialog(vd.gui, "Group", "")
	dlgEditor.multiline = false
	dlgEditor.onFinish = func(ss []string) {
		ds.groupTaggedItemsUnder(ss[0])
		updateMainPane()
	}
}

func cmdUngroupItems() {
	ds.ungroupItems()
	updateMainPane()
}

func cmdNextItem() {
	ds.nextItem()
	updateMainPane()
}

func cmdPrevItem() {
	ds.prevItem()
	updateMainPane()
}

func cmdFirstItem() {
	ds.firstItem()
	updateMainPane()
}

func cmdLastItem() {
	ds.lastItem()
	updateMainPane()
}

func cmdDescend() {
	ds.focusDescend()
	updateMainPane()
}

func cmdAscend() {
	ds.focusAscend()
	updateMainPane()
}

func cmdSaveData() {
	ds.save()
}

func cmdLoadData() {
	ds.load()
}

func cmdSetUserTarget() {
	ds.SetUserTarget()
	// No need to update pane.
}

func cmdMoveToTarget(t *Target) {
	ds.MoveToTarget(t)
	updateMainPane()
}

func cmdGoToUserTarget() {
	ds.GoToUserTarget()
	updateMainPane()
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
