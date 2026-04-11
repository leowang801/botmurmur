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
	"fmt"
	"os"
)

const version = "0.0.1-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scan":
		fmt.Fprintln(os.Stderr, "botmurmur scan: not yet implemented (Day 2 lands the Linux lister)")
		os.Exit(1)
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

func usage() {
	fmt.Fprintln(os.Stderr, `botmurmur — agent-semantic process scanner

Usage:
  botmurmur scan     one-shot JSON inventory of running agents
  botmurmur watch    poll every 30s, print diffs on change
  botmurmur version  print version and exit
  botmurmur help     print this help

See github.com/leowang801/botmurmur for the full design doc.`)
}
