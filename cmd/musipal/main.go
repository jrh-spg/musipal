// Command musipal is a CLI/TUI music player.
package main

import (
	"flag"
	"fmt"
	"os"

	"musipal/internal/tui"
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
