// Command botmurmur is an agent-semantic process scanner. Given a running
// process, it determines whether it is an AI agent, which framework it uses,
// what credentials it holds in env vars, and what tools it has access to.
//
// Usage:
//
//	botmurmur scan   # one-shot JSON inventory to stdout
//	botmurmur watch  # poll every 30s, print diff on change
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/leowang801/botmurmur/cmd/scan"
	"github.com/leowang801/botmurmur/internal/proc"
)

const version = "0.0.1-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scan":
		if err := runScan(); err != nil {
			fmt.Fprintln(os.Stderr, "botmurmur scan:", err)
			os.Exit(1)
		}
	case "watch":
		fmt.Fprintln(os.Stderr, "botmurmur watch: not yet implemented (Day 3)")
		os.Exit(1)
	case "version", "--version", "-v":
		fmt.Println("botmurmur", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "botmurmur: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// runScan executes the two-phase scan pipeline and JSON-encodes the result
// to stdout. Partial failures are already captured as inline warnings in the
// Scan struct, so this function returns non-nil only on fatal errors.
func runScan() error {
	lister := proc.NewLister()
	result, err := scan.Run(lister)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func usage() {
	fmt.Fprintln(os.Stderr, `botmurmur — agent-semantic process scanner

Usage:
  botmurmur scan     one-shot JSON inventory of running agents
  botmurmur watch    poll every 30s, print diffs on change
  botmurmur version  print version and exit
  botmurmur help     print this help

See github.com/leowang801/botmurmur for the full design doc.`)
}
