package main

import (
	"fmt"
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

func colorString(s string, fg, bg int, extra string) string {
	return fmt.Sprintf("\033[%d;%d%vm%v\033[0m", bg, fg, extra, s)
}

func setTitle(s string) {
	fmt.Printf("\033]0;%s\007", s)
}

// vim: fdm=syntax
