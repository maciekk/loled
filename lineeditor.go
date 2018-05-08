package main

import (
	"strings"
	"unicode"

	"github.com/jroimartin/gocui"
)

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
