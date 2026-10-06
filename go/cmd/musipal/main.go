// Command musipal is the Go port of the Python musipal CLI/TUI music player.
package main

import (
	"flag"
	"fmt"
	"os"

	"musipal-go/internal/tui"
)

func main() {
	libraryRoot := flag.String("library-root", "", "Override library root path (defaults to config)")
	flag.Parse()

	app, err := tui.New(*libraryRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "musipal:", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "musipal:", err)
		os.Exit(1)
	}
}
