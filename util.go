package main

import (
	"bufio"
	"os"
)

func readString() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\r')
	if text[len(text)-1] == '\n' || text[len(text)-1] == '\r' {
		text = text[:len(text)-1]
	}
	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
