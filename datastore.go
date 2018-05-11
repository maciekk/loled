package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

// Returns the created node.
func (ds *dataStore) appendItem(s string) *node {
	n := node{
		s,                // payload
		ds.currentList,   // parent
		make([]*node, 0), // sublist
		false,            // tagged
	}
	i := ds.currentItemIndex()
	ds.currentList.insertKid(i+1, &n)

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
	i := ds.currentItemIndex()
	ds.currentList.removeKid(i)
	kids := &ds.currentList.sublist

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
	ds.currentItem.parent = t.list

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

	// Remove fold node from kids.
	i := ds.currentItemIndex()
	ds.currentList.removeKid(i)
	// And we let ds.currentItem node just get garbage collected after
	// this.

	// Add in the subkids at same index.
	// Iterate in reverse so that end ordering stays unchanged.
	subkids := ds.currentItem.sublist
	for j := len(subkids) - 1; j >= 0; j-- {
		ds.currentList.insertKid(i, subkids[j])
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
	ds.currentList.insertKid(i, &nFold)

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

// vim: fdm=syntax
