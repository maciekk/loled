// List of List (LOL) EDitor
//
// TODO
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
//
// NOTES
// - See https://appliedgo.net/tui/ for review of potential TUIs to use.

package main

import (
	"bufio"
	"flag"
	"fmt"
	termbox "github.com/nsf/termbox-go"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
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
	if ds.idCurrentItem >= 0 {
		kids := &ds.currentList().sublist
		for i, id := range *kids {
			if id == ds.idCurrentItem {
				// First, update the current item.
				if len(*kids) == 1 {
					ds.idCurrentItem = -1 // we are removing last item on list
				} else if i > 0 {
					ds.idCurrentItem = (*kids)[i-1] // select previous item
				} else {
					// deleting first item on list, but there are
					// others
					ds.idCurrentItem = (*kids)[1]
				}

				// Next, remove trace of the node.
				*kids = append((*kids)[:i], (*kids)[i+1:]...)
				delete(ds.nodes, id)

				return
			}
		}
	}
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
	fmt.Printf("Saved to %q.\n", *filename)
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

// Stolen from Termbox's boxed demo app.
func tbprint(x, y int, fg, bg termbox.Attribute, msg string) {
	for _, c := range msg {
		termbox.SetCell(x, y, c, fg, bg)
		x += 1 // TODO: at some point consider rune width
	}
}

// Stolen from Termbox's boxed demo app.
func fill(x, y, w, h int, cell termbox.Cell) {
	for ly := 0; ly < h; ly++ {
		for lx := 0; lx < w; lx++ {
			termbox.SetCell(x+lx, y+ly, cell.Ch, cell.Fg, cell.Bg)
		}
	}
}

func redrawAll(ds *dataStore) {
	const coldef = termbox.ColorDefault
	termbox.Clear(coldef, coldef)
	//w, h := termbox.Size()

	// current line being printed
	lidx := 0

	n := ds.currentList()

	title := fmt.Sprintf("[[ %v ]]", n.label)
	tbprint(0, lidx, termbox.ColorWhite, termbox.ColorBlack, title)
	lidx += 1
	tbprint(0, lidx, coldef, coldef, strings.Repeat("=", len(title)))
	lidx += 1

	for _, item := range n.sublist {
		pfx := pfxItem
		if item == ds.idCurrentItem {
			pfx = pfxFocusedItem
		}
		sfx := ""
		if len(ds.nodes[item].sublist) > 0 {
			sfx = " ..."
		}
		tbprint(0, lidx, coldef, coldef, pfx+ds.nodes[item].label+sfx)
		lidx += 1
	}

	termbox.Flush()
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

////////////////////////////////////////
// COMMANDS

func cmdAddItems(ds *dataStore) {
	for {
		fmt.Print("Enter item: ")
		text := strings.TrimRight(readString(), whitespace)
		if text == "" {
			break
		}
		ds.appendItem(text)
	}
}

func cmdDeleteItem(ds *dataStore) {
	ds.deleteItem()
}

func cmdMoveItem(ds *dataStore) {
	idx := ds.currentItemIndex()
	max_idx := len(ds.currentList().sublist) - 1

	// Select WHERE TO move item.
	key := readKey()
	switch key {
	case 'k':
		if idx > 0 {
			ds.moveItemToIndex(idx - 1)
		}
	case 'K',
		'0':
		if idx > 0 {
			ds.moveItemToIndex(0)
		}
	case 'j':
		if idx < max_idx {
			ds.moveItemToIndex(idx + 1)
		}
	case 'J',
		'-':
		if idx < max_idx {
			ds.moveItemToIndex(max_idx)
		}
	}
}

func cmdNextItem(ds *dataStore) {
	ds.nextItem()
}

func cmdPrevItem(ds *dataStore) {
	ds.prevItem()
}

func cmdDescend(ds *dataStore) {
	ds.focusDescend()
}

func cmdAscend(ds *dataStore) {
	ds.focusAscend()
}

func cmdReplaceItem(ds *dataStore) {
	fmt.Printf("Replace with: ")
	ds.replaceItem(readString())
}

func cmdSaveData(ds *dataStore) {
	ds.save()
}

func cmdLoadData(ds *dataStore) {
	ds.load()
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

func main() {
	flag.Parse()

	var ds dataStore

	if _, err := os.Stat(*filename); err == nil {
		ds.load()
	} else {
		fmt.Printf("Unable to stat %q; creating empty dataStore instead.\n", *filename)
		ds.init()
	}

	// set up UI
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputEsc)

	redrawAll(&ds)

mainloop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			// TODO: need to combine Key & Ch switches
			switch ev.Key {
			case termbox.KeyEsc:
				break mainloop
			case termbox.KeyEnter:
				cmdDescend(&ds)
			default:
				// if no special key, switch on character
				switch ev.Ch {
				case 'q':
					break mainloop
				case 'a':
					cmdAddItems(&ds)
				case 'D':
					cmdDeleteItem(&ds)
				case 'm':
					cmdMoveItem(&ds)
				case 'j':
					cmdNextItem(&ds)
				case 'k':
					cmdPrevItem(&ds)
				case '>':
					cmdDescend(&ds)
				case 'r':
					cmdReplaceItem(&ds)
				case 'u',
					'<':
					cmdAscend(&ds)
				case 'S':
					cmdSaveData(&ds)
				case 'L':
					cmdLoadData(&ds)
				default:
					// TODO: print something to echo area
					fmt.Println("  unknown command")
				}
			}
		case termbox.EventError:
			panic(ev.Err)
		}
		redrawAll(&ds)
	}
	fmt.Printf("Quitting... ")
	if ds.dirty {
		fmt.Printf("save first? [y/n] ")
		x := readString()
		if strings.HasPrefix(x, "y") {
			ds.save()
		}
	}
}
