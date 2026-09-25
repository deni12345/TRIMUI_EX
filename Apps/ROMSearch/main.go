package main

import (
	"fmt"
	"os"
)

func main() {
	if e := runUI(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
