package main

import (
	"fmt"
	"io"
	"os"
)

func Log(s string, a ...interface{}) {
	var f io.Writer
	if vd.paneMessage != nil {
		f = vd.paneMessage
	} else {
		f = os.Stdout
	}
	fmt.Fprintf(f, s+"\n", a...)
}

// vim: fdm=syntax
