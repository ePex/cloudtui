// Command tui launches the cloudtui terminal UI.
package main

import (
	"fmt"
	"os"

	"github.com/ePex/cloudtui/tui/internal/app"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func main() {
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: loading config: %v (using defaults)\n", err)
		cfg = config.Default()
	}
	if err := app.New(cfg).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
