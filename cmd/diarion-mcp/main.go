package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "0.0.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("diarion-mcp %s\n", version)
		os.Exit(0)
	}

	// M8 will replace this with the MCP server implementation.
	fmt.Fprintln(os.Stderr, "diarion-mcp: M0 stub. MCP server not yet implemented.")
	os.Exit(1)
}
