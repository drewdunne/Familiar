package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: familiar <command>")
		fmt.Println("Commands:")
		fmt.Println("  serve    Start the webhook server")
		fmt.Println("  version  Print version information")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Println("Starting server... (not implemented)")
	case "version":
		fmt.Println("familiar v0.1.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
