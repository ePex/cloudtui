// Command tui launches the cloudtui terminal UI.
package main

import (
	"fmt"
	"os"

	"github.com/ePex/cloudtui/tui/internal/app"
)

func main() {
	if err := app.New().Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
