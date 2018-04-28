// List of List (LOL) EDitor
//
// TODO
// - add "item move" commands: up, down, top, bottom
// - add 'P'rint command, which prints the whole recursive tree.
// - start using ncurses, so can do side-by-sides, etc.
// - explore going back to 'sublist' being []*node, rather than []int of IDs
// - start using the list for actual TODOs
// - list of recent *.lol files edited should itself be a list under root
// - provide convenience routine to get at sublist based on node ID
// - every save, renumber node IDs to compact them, to remove holes.
// - do "safe" saves, in that first write to temp file, and only if success,
//   rename and replace old version

package main

import (
	"bufio"
	"flag"
	"fmt"
	// If ever want to use ncurses, module below.
	//"github.com/rthornton128/goncurses"
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

func (ds *dataStore) sprintCurrentList() []string {
	var res []string
	n := ds.currentList()

	title := fmt.Sprintf("[[ %v ]]", n.label)
	res = append(res, title)
	res = append(res, strings.Repeat("=", len(title)))
	for _, item := range n.sublist {
		pfx := pfxItem
		if item == ds.idCurrentItem {
			pfx = pfxFocusedItem
		}
		sfx := ""
		if len(ds.nodes[item].sublist) > 0 {
			sfx = " ..."
		}
		res = append(res, pfx+ds.nodes[item].label+sfx)
	}
	return res
}

func (ds *dataStore) printCurrentList() {
	for _, s := range ds.sprintCurrentList() {
		fmt.Println(s)
	}
}

func (ds *dataStore) appendItem(s string) {
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
			ds.freeID += 1
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

func readString() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	if text[len(text)-1] == '\n' {
		text = text[:len(text)-1]
	}
	return text
}

func clearScreen() {
	cmd := exec.Command("/usr/bin/clear") // TODO: make OS-agnostic
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func cmdPrint(ds *dataStore) {
	clearScreen()
	ds.printCurrentList()
}

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
	cmdPrint(ds)
}

func cmdNextItem(ds *dataStore) {
	ds.nextItem()
	cmdPrint(ds)
}

func cmdPrevItem(ds *dataStore) {
	ds.prevItem()
	cmdPrint(ds)
}

func cmdDescend(ds *dataStore) {
	ds.focusDescend()
	cmdPrint(ds)
}

func cmdAscend(ds *dataStore) {
	ds.focusAscend()
	cmdPrint(ds)
}

func cmdReplaceItem(ds *dataStore) {
	fmt.Printf("Replace with: ")
	ds.replaceItem(readString())
	cmdPrint(ds)
}

func cmdQuit(ds *dataStore) {
	fmt.Printf("Quitting... ")
	if ds.dirty {
		fmt.Printf("save first? [y/n] ")
		x := readString()
		if strings.HasPrefix(x, "y") {
			ds.save()
		}
	}
	fmt.Println()
	os.Exit(0)
}

func cmdSaveData(ds *dataStore) {
	ds.save()
}

func cmdLoadData(ds *dataStore) {
	ds.load()
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

func pop(l *[]string) string {
	v := (*l)[0]
	*l = (*l)[1:]
	return v
}

func pushBack(l *[]string, s string) {
	*l = append(*l, s)
}

func main() {
	flag.Parse()

	var ds dataStore

	if _, err := os.Stat(*filename); err == nil {
		ds.load()
	} else {
		fmt.Printf("Unable to stat %q; creating empty dataStore instead.\n", *filename)
		ds.init()
	}
	ds.printCurrentList()

	// interaction loop
	for {
		fmt.Printf(cmdPrompt)
		//switch readString()[0] {
		//switch stdscr.GetChar() {
		key := readKey()
		// Echo the key, as echo is turned off.
		// FWIW, longer term we may print something more.
		fmt.Printf("%c\n", key)

		switch key {
		case 'q':
			cmdQuit(&ds)
		case 'p':
			cmdPrint(&ds)
		case 'a':
			cmdAddItems(&ds)
		case 'd':
			cmdDeleteItem(&ds)
		case 'j':
			cmdNextItem(&ds)
		case 'k':
			cmdPrevItem(&ds)
		case '\n',
			'>':
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
			fmt.Println("  unknown command")
		}
	}
}
