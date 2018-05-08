package main

import (
	"fmt"

	"github.com/jroimartin/gocui"
)

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
