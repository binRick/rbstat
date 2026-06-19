// Command rbstat is a Go clone of the classic Linux dstat resource monitor.
package main

import (
	"fmt"
	"os"
)

func main() {
	cfg, act, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "rbstat:", err)
		fmt.Fprintln(os.Stderr, "Try 'rbstat --help' for more information.")
		os.Exit(1)
	}
	switch act {
	case actionVersion:
		fmt.Printf("rbstat %s\n", version)
		return
	case actionHelp:
		printHelp()
		return
	case actionList:
		for _, n := range allNames {
			fmt.Println(n)
		}
		return
	}
	if cfg.usingDefault {
		fmt.Fprintln(os.Stderr, "You did not select any stats, using -cdngy by default.")
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "rbstat:", err)
		os.Exit(1)
	}
}
