package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--install-worker" {
		if e := runInstallWorker(); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	if e := runUI(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
