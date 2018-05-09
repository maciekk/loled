package main

import (
	"strings"
)

////////////////////////////////////////
// COMMANDS

func cmdAddItems() {
	dlgEditor := dialog(vd.gui, "Add", "", true)
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
	if ds.currentItem == nil {
		return
	}
	dlgEditor := dialog(vd.gui, "Replace", ds.currentItem.label, false)
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
	dlgEditor := dialog(vd.gui, "Fold", "", false)
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

// vim: fdm=syntax
