// List of Lists (LOL) EDitor
//
// TODO
// - Done & Trash should move such that most recent is topmost in their lists
// - add Status window, on top of log window: logo + stats about datastore
// - bug: segfaults on empty *.lol file
// - bug: on fold, when items at start of list, visible cursor not adjusted.
// - need a reliable way to auto-set 'dirty'; I often forget in new code.
// - rather than printing "root", the root node should be labeled with
//   filename being edited.
// - clean up finally the singletons (vd & ds), and distribute methods better!
// - now that can hop with random-access, need "go back" command
// - maybe we should have ability to memorize and jump to a memorized location
//   (e.g., Vim's 'm' and apostrophe normal mode commands)
// - visual revamp
//   - better method for printing color strings, w/o ANSII escapes
// - fix gocui off-by-one bug w/256 color setting in SelFgCol and SelBgCol
//   (possibly bug in termbox-go underneath gocui)
// - have "Pull from Target" command, reverse of 'M'ove
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
	LABEL_TRASH         = "[[TRASH]]"
	LABEL_DONE          = "[[DONE]]"
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
	// node content payload
	label string
	// parent node; nil if no parent (should be true only for root node)
	parent *node
	// list of children
	sublist []*node
	// Is it tagged?
	tagged bool
}

// A "pointer" into the mass structure of list-of-lists. Identifies an item
// position, much like a cursor.
type Target struct {
	list  *node // list within which Target lies
	index int   // Target points at item at this index.
	// Target really occupies space BETWEEN characters, which gives two
	// possibilities when pointing at an item using a list index: just
	// before it, or just after.
	before bool
}

// The "Model" component of MVC framework.
type dataStore struct {
	// Root node
	root *node

	// Current list and currently selected item in it.
	//
	// NOTE: it is possible the list is empty, and thus does not have a
	// selected item. In that case currentItem is nil.
	// TODO: this too should be a Target (and probably name 'cursor').
	currentList *node
	currentItem *node

	// Indicates if data has been modified, and needs to be saved.
	dirty bool

	// User-defined targets.
	//
	// NOTE: in future we will have multiple; these will be like  "marks"
	// in Vim.
	Mark Target

	// Pre-defined special targets.
	// NOTE: using * so that able to differentiate uninitialized Target.
	markTrash *Target // Where deleted items are moved.
	markDone  *Target // Where DONE items are moved.
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
	max_idx := len(ds.currentList.sublist) - 1
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
		ds.setCurrentItemUsingIndex(new_idx)
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
	case ch == 'J' || ch == '$' || ch == '-' || ch == 'G':
		cmdLastItem()
	case ch == 'K' || ch == '0' || ch == 'g':
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
	case ch == 'f':
		cmdFoldItems()
	case ch == 'F':
		cmdUnfoldItems()
	case ch == 't':
		cmdSetUserTarget()
	case ch == 'T':
		cmdGoToUserTarget()
	case ch == 'M':
		cmdMoveCurrentItemToTarget(&ds.Mark)
	case ch == 'd':
		cmdMoveToDone()
	case ch == 'D':
		cmdMoveToTrash()
	case ch == 'X':
		cmdExpungeTrash()
		/*
			case ch == 'q':
				quit()
		*/
	default:
		fmt.Printf("\007") // BELL
	}
}

// Defines callback type for dialog box; it will be called once entry in
// dialog box completes. The possibly multi-line string from dialog box is
// split on newlines and fed to this callback.
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
	// (i.e., origin has moved)? TODO: fix
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
	if ds.root == nil {
		ds.root = &node{
			"root",
			nil, // parent pointer (i.e., none)
			make([]*node, 0),
			false, // tagged
		}
	}

	// Set temporarily, for potential insertions.
	ds.currentList = ds.root
	rootkids := ds.root.sublist

	// Ensure DONE exists.
	if ds.markDone == nil {
		// First, need cursor at end of root list.
		if len(rootkids) > 0 {
			ds.currentItem = rootkids[len(rootkids)-1]
		} else {
			ds.currentItem = nil
		}
		n := ds.appendItem(LABEL_DONE)
		ds.markDone = &Target{n, 0, true} // always just before first item
	}

	// Ensure Trash exists.
	if ds.markTrash == nil {
		// First, need cursor at end of root list.
		if len(rootkids) > 0 {
			ds.currentItem = rootkids[len(rootkids)-1]
		} else {
			ds.currentItem = nil
		}
		n := ds.appendItem(LABEL_TRASH)
		ds.markTrash = &Target{n, 0, true} // always just before first item
	}

	// Reset cursor.
	ds.currentList = ds.root
	if len(rootkids) > 0 {
		ds.currentItem = rootkids[0]
	} else {
		// invalid == no current item
		ds.currentItem = nil
	}
}

// Sets current item and updates the UI selection bar to it.
func (ds *dataStore) setCurrentItemUsingIndex(idx int) {
	if ds.currentList == nil {
		// We do not Log() as this is usually not an action explicitly
		// triggered by the user, but rather a support function. They may
		// call it automatically even when not suitable (e.g., init time).
		return
	}
	items := ds.currentList.sublist
	if idx < 0 || len(items) == 0 {
		// no selected item
		ds.currentItem = nil
		if vd.paneMain != nil {
			vd.paneMain.Highlight = false
		}
		return
	}
	if idx > len(items)-1 {
		panic(fmt.Sprintf(
			"Bad index for setCurrentItemUsingIndex(): %v (len = %v)\n",
			idx, len(items)))
		if vd.paneMain != nil {
			vd.paneMain.Highlight = false
		}
		return
	}

	ds.currentItem = items[idx]
	if vd.paneMain != nil {
		// +2 offset due to list title and underline.
		vd.paneMain.SetCursor(0, idx+2)
		vd.paneMain.Highlight = true
	}
}

func (ds *dataStore) indexOfItem(n *node) int {
	l := ds.currentList
	if l == nil {
		Log("indexOfItem(%v) called but no current list.", n)
		return -1
	}
	if l.sublist == nil {
		Log("indexOfItem(%v) called but current list has no sublist.", n)
		return -1
	}
	for i, k := range ds.currentList.sublist {
		if k == n {
			return i
		}
	}
	// Didn't find it.
	return -1
}

func (ds *dataStore) currentItemIndex() int {
	if ds.currentItem == nil {
		// Error.
		return -1
	}

	for i, k := range ds.currentList.sublist {
		if k == ds.currentItem {
			return i
		}
	}

	panic("Current item not on current list.")
}

func insertKid(newkid *node, kids *[]*node, pos int) {
	// Sigh, Go has gross item insertion.

	*kids = append(*kids, nil) // extend length by 1
	copy((*kids)[pos+1:], (*kids)[pos:])
	(*kids)[pos] = newkid
}

// Returns the created node.
func (ds *dataStore) appendItem(s string) *node {
	n := node{
		s,                // payload
		ds.currentList,   // parent
		make([]*node, 0), // sublist
		false,            // tagged
	}
	kids := &ds.currentList.sublist
	i := ds.currentItemIndex()
	insertKid(&n, kids, i+1)

	// Make the latest node the current one.
	ds.setCurrentItemUsingIndex(i + 1)

	ds.dirty = true

	return &n
}

// Replace the current item's label with the provided string.
func (ds *dataStore) replaceItem(s string) {
	if ds.currentItem == nil {
		// No current item.
		return
	}
	ds.currentItem.label = s
	ds.dirty = true
}

func (ds *dataStore) toggleItem() {
	if ds.currentItem == nil {
		// no items
		return
	}

	ds.currentItem.tagged = !ds.currentItem.tagged
}

func (ds *dataStore) toggleAllItems() {
	if ds.currentItem == nil {
		// no items
		return
	}

	newval := !ds.currentItem.tagged
	for _, n := range ds.currentList.sublist {
		n.tagged = newval
	}

}

func (ds *dataStore) SetUserTarget() {
	ds.Mark.list = ds.currentList
	ds.Mark.index = ds.currentItemIndex()
	// User marks are ALWAYS 'after', at least for now.
	// TODO: theoretically could have different key bindings for 'before'
	// and 'after'. However, not clear how to indicate visually which one
	// we have, on GoToUserTarget().
	ds.Mark.before = false
	Log("Target set.")
}

func (ds *dataStore) GoToUserTarget() {
	if ds.Mark.list == nil {
		Log("Target not set.")
		return
	}
	ds.currentList = ds.Mark.list
	// Check if Target is on a list with no items.
	if ds.Mark.index == -1 {
		ds.currentItem = nil
	} else {
		ds.currentItem = ds.currentList.sublist[ds.Mark.index]
	}
	ds.setCurrentItemUsingIndex(ds.Mark.index)
	Log("Jumped to Target.")
}

func (ds *dataStore) ExpungeTrash() {
	if len(ds.markTrash.list.sublist) > 0 {
		ds.markTrash.list.sublist = ds.markTrash.list.sublist[0:0]
		ds.dirty = true
	}
}

// Move cursor to given target 't'.
// Also, if move did occur, advances the target to point at moved item.
func (ds *dataStore) MoveCurrentItemToTarget(t *Target) {
	// Check that there is anything to do.
	if ds.currentList == nil || ds.currentItem == nil {
		Log("No current list or item.")
		return
	}

	// First, remove item from current list.
	// TODO: is there a shared function we could use? Share some code with
	// insertKid()?
	kids := &ds.currentList.sublist
	i := ds.currentItemIndex()
	*kids = append((*kids)[:i], (*kids)[i+1:]...)

	// Find successor, if any.
	var newCurrentItem *node
	if i >= len(*kids) {
		i = len(*kids) - 1
	}
	if i < 0 {
		// No more items left on list.
		newCurrentItem = nil
	} else {
		newCurrentItem = (*kids)[i]
	}

	// Now place it at Target
	kids = &t.list.sublist
	if len(*kids) == 0 {
		*kids = []*node{ds.currentItem}
	} else {
		*kids = append(*kids, nil) // extend length by 1
		i = t.index
		if !t.before {
			i += 1
		}
		// There is a second part to shift only if item to insert is
		// not meant as last item.
		if i < len(*kids)-1 {
			copy((*kids)[i+1:], (*kids)[i:])
		}
		if i > len(*kids)-1 {
			Log("WARNING: Target pointing beyond list (idx=%v vs maxidx=%v).",
				i, len(*kids)-1)
			i = len(*kids) - 1
		}
		(*kids)[i] = ds.currentItem
	}

	// Maybe advance Target index, depending on type of Target. Behaviour
	// is determined by what the end effect is of moving multiple items
	// using these Targets:
	// "Before" Targets: desired effect = reverse chronological addition
	// "After" Targets: desired effect = chronological addition
	if !t.before {
		t.index += 1
	}

	// Finally make sure current item is its former successor.
	ds.currentItem = newCurrentItem

	ds.dirty = true
}

func (ds *dataStore) unfoldItems() {
	if ds.currentItem == nil || len(ds.currentItem.sublist) < 1 {
		Log("Cannot unfold, item invalid or has no sublist.")
		return
	}

	kids := &ds.currentList.sublist
	subkids := ds.currentItem.sublist

	// Remove fold node from kids.
	// TODO use shared fn for this, akin to insertKid().
	i := ds.currentItemIndex()
	*kids = append((*kids)[:i], (*kids)[i+1:]...)
	// And we let ds.currentItem node just get garbage collected after
	// this.

	// Add in the subkids at same index.
	// Iterate in reverse so that end ordering stays unchanged.
	for j := len(subkids) - 1; j >= 0; j-- {
		insertKid(subkids[j], kids, i)
	}

	// Update current item.
	ds.setCurrentItemUsingIndex(i)

	ds.dirty = true
}

func (ds *dataStore) foldTaggedItemsUnder(name string) {
	kids := &ds.currentList.sublist
	listTagged := []*node{}
	listUntagged := []*node{}
	// Find the tagged item that is highest on list; it will determine
	// fold node placement.
	idxFirstTagged := -1 // not set
	for i, k := range *kids {
		if k.tagged {
			// TODO: do we really want to clear the tag bit?
			k.tagged = false
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

	// Create new node for fold.
	nFold := node{
		name,           // payload
		ds.currentList, // parent
		listTagged,     // sublist
		false,          // tagged
	}

	// Insert the new node into current list.
	i := idxFirstTagged
	if i > len(*kids) {
		i = len(*kids)
	}
	insertKid(&nFold, kids, i)

	// Adjust current item.
	ds.currentItem = &nFold

	ds.dirty = true
}

// Workhorse for the 'm'ove command, which moves item up/down within current
// list (vs 'Move' command which moves to Target).
func (ds *dataStore) moveItemToIndex(idxNew int) {
	idx := ds.currentItemIndex()
	if idx == idxNew {
		return
	}

	sublistNew := make([]*node, 0)
	for _, kid := range ds.currentList.sublist {
		if kid == ds.currentItem {
			// The currentItem is the one being moved, it will be put
			// on either list by other code below.
			continue
		}
		// New sublist is long-enough that we can place currentItem at
		// right spot.
		if len(sublistNew) == idxNew {
			sublistNew = append(sublistNew, ds.currentItem)
		}
		sublistNew = append(sublistNew, kid)
	}
	// If being placed as last item.
	if len(sublistNew) == idxNew {
		sublistNew = append(sublistNew, ds.currentItem)
	}
	ds.currentList.sublist = sublistNew
	ds.dirty = true
}

// Advance currentItem.
func (ds *dataStore) nextItem() {
	list := ds.currentList.sublist
	for i, kid := range list {
		if kid == ds.currentItem {
			if i < len(list)-1 {
				ds.setCurrentItemUsingIndex(i + 1)
			}
			return
		}
	}
}

// Back up currentItem.
func (ds *dataStore) prevItem() {
	list := ds.currentList.sublist
	for i, kid := range list {
		if kid == ds.currentItem {
			if i > 0 {
				ds.setCurrentItemUsingIndex(i - 1)
			}
			return
		}
	}
}

func (ds *dataStore) firstItem() {
	ds.setCurrentItemUsingIndex(0)
}

func (ds *dataStore) lastItem() {
	l := &ds.currentList.sublist
	ds.setCurrentItemUsingIndex(len(*l) - 1)
}

func (ds *dataStore) focusDescend() {
	if ds.currentItem != nil {
		ds.currentList = ds.currentItem
		if len(ds.currentList.sublist) > 0 {
			ds.setCurrentItemUsingIndex(0)
		} else {
			// < 0 means no item selected
			ds.setCurrentItemUsingIndex(-1)
		}
		// TODO: push this off to cmd*()
		//vd.paneMain.Title = ds.currentList().label
	}
	// Else do nothing; < 0 implies ds.currentList does not have items.
}

func (ds *dataStore) focusAscend() {
	if ds.currentList.parent == nil {
		// Nothing to do if already at a root.
		return
	}
	newCurrentItem := ds.currentList
	ds.currentList = ds.currentList.parent
	ds.setCurrentItemUsingIndex(ds.indexOfItem(newCurrentItem))

	// TODO: push this off to cmd*()
	//vd.paneMain.Title = ds.currentList().label
}

func mapToId(n *node, m *map[*node]int, freeId *int) {
	if _, ok := (*m)[n]; ok {
		// Node already in map.
		return
	}
	// Grab fresh ID before visiting kids.
	(*m)[n] = *freeId
	*freeId += 1
	for _, kid := range n.sublist {
		mapToId(kid, m, freeId)
	}
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

	// Pre-work: map each node to a unique ID.
	nodeMap := make(map[*node]int)
	freeId := 1
	mapToId(ds.root, &nodeMap, &freeId)

	// First, write out special node ids.
	if ds.markDone != nil {
		idDone := nodeMap[ds.markDone.list]
		f.WriteString(fmt.Sprintf("DONE %v\n", idDone))
	}
	if ds.markTrash != nil {
		idTrash := nodeMap[ds.markTrash.list]
		f.WriteString(fmt.Sprintf("TRASH %v\n", idTrash))
	}

	// Finally, write out nodes in breadth first order.
	nToDo := []*node{ds.root}
	var n *node
	for len(nToDo) > 0 {
		n, nToDo = nToDo[0], nToDo[1:]
		f.WriteString(fmt.Sprintf("node %v\n", nodeMap[n]))
		f.WriteString(fmt.Sprintf("%s\n", n.label))
		// TODO: get rid of trailing space after last item; use some
		// join()
		for _, child := range n.sublist {
			f.WriteString(fmt.Sprintf("%v ", nodeMap[child]))
		}
		// NOTE: if no children, will result in blank line.
		// (intentional)
		f.WriteString("\n")

		nToDo = append(nToDo, n.sublist...)
	}

	ds.dirty = false
	Log("Saved to %q.", *filename)
}

func (ds *dataStore) load() {
	// First wipe any data we have.
	ds.root = nil

	// We will need to build up a map, to better link things.
	type nodeData struct {
		n    *node
		kids []int
	}
	nodeMap := make(map[int]nodeData)

	data, err := ioutil.ReadFile(*filename)
	if err != nil {
		fmt.Printf("Error reading %q: %q\n", filename, err)
		return
	}

	var idDone = -1
	var idTrash = -1

	lines := strings.Split(string(data), "\n")

	var l string
	for {
		// If ran out of input, we are done.
		if len(lines) < 1 {
			break
		}

		l = pop(&lines)

		// Skip any blank lines.
		if strings.Trim(l, whitespace) == "" {
			continue
		}

		// First watch for special nodes.
		if strings.HasPrefix(l, "DONE ") {
			l = l[5:]
			id, err := strconv.Atoi(l)
			if err != nil {
				panic(err)
			}
			idDone = id
			continue
		}
		if strings.HasPrefix(l, "TRASH ") {
			l = l[6:]
			id, err := strconv.Atoi(l)
			if err != nil {
				panic(err)
			}
			idTrash = id
			continue
		}

		// If not any above, then it should be a node definition.
		if !strings.HasPrefix(l, "node ") {
			fmt.Printf("Format error: expected node #, got %q.\n", l)
			return
		}

		l = l[5:] // Strip "node ".
		id, err := strconv.Atoi(l)

		if err != nil {
			panic(err)
		}

		label := pop(&lines)

		l = strings.Trim(pop(&lines), whitespace)
		var idKids []int
		if len(l) > 0 {
			// SOME kids
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
			label,
			nil,   // parent; TBD
			nil,   // kids; TBD
			false, // tagged
		}
		nodeMap[id] = nodeData{
			&n,
			idKids,
		}
		if label == "root" && (id == 0 || id == 1) {
			ds.root = &n
		}
	}

	// Readjust parent & kid pointers.
	for _, ndata := range nodeMap {
		for _, idKid := range ndata.kids {
			ndata.n.sublist = append(ndata.n.sublist, nodeMap[idKid].n)
			nodeMap[idKid].n.parent = ndata.n
		}
	}

	// Handle special Targets.
	if idDone > 0 {
		n := nodeMap[idDone].n
		ds.markDone = &Target{n, 0, true}
	}
	if idTrash > 0 {
		n := nodeMap[idTrash].n
		ds.markTrash = &Target{n, 0, true}
	}

	ds.currentList = ds.root
	ds.currentItem = nil
	if len(ds.root.sublist) > 0 {
		ds.setCurrentItemUsingIndex(0)
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
		if len(ds.currentList.sublist) > 0 {
			ds.setCurrentItemUsingIndex(0)
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

	n := ds.currentList

	if n == nil {
		panic("currentList not found!")
	}

	title := fmt.Sprintf("▶ %v", n.label) // TODO: add more info
	fmt.Fprintln(vd.paneMain, title)
	//fmt.Fprintln(vd.paneMain, colorString(title, BG_BLACK, FG_WHITE, ";1"))
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
	dlgEditor := dialog(vd.gui, "Replace", ds.currentItem.label)
	dlgEditor.multiline = false
	dlgEditor.onFinish = func(ss []string) {
		ds.replaceItem(ss[0])
		updateMainPane()
	}
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

func cmdFoldItems() {
	dlgEditor := dialog(vd.gui, "Fold", "")
	dlgEditor.multiline = false
	dlgEditor.onFinish = func(ss []string) {
		ds.foldTaggedItemsUnder(ss[0])
		updateMainPane()
	}
}

func cmdUnfoldItems() {
	ds.unfoldItems()
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

func cmdMoveCurrentItemToTarget(t *Target) {
	ds.MoveCurrentItemToTarget(t)
	updateMainPane()
}

func cmdMoveToDone() {
	ds.MoveCurrentItemToTarget(ds.markDone)
	updateMainPane()
}

func cmdMoveToTrash() {
	ds.MoveCurrentItemToTarget(ds.markTrash)
	updateMainPane()
}

func cmdGoToUserTarget() {
	ds.GoToUserTarget()
	updateMainPane()
}

func cmdExpungeTrash() {
	ds.ExpungeTrash()
	// Strictly only necessary if:
	// - Trash is currently displayed
	// - root is being displayed ("has items" indicator should disappear)
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
