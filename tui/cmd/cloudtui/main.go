// Command tui launches the cloudtui terminal UI.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ePex/cloudtui/tui/internal/app"
	"github.com/ePex/cloudtui/tui/internal/config"
)

// version is overridden via -ldflags at release build time (see
// .goreleaser.yaml); a locally-built binary reports "dev".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("cloudtui " + version)
		return
	}

	var logWriter io.Writer = io.Discard
	if f := openLogFile(); f != nil {
		defer f.Close()
		logWriter = f
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, nil)))

	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: loading config: %v (using defaults)\n", err)
		cfg = config.Default()
	}

	cfgPath, err := config.DefaultPath()
	if err != nil {
		cfgPath = "(unresolved)"
	}
	slog.Info("startup", "config", cfgPath, "theme", cfg.Theme)

	if err := app.New(cfg).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// openLogFile creates ~/.cloudtui/cloudtui.log (truncating any previous
// content) and returns the open file. Returns nil on any error so the caller
// can fall back to discarding log output rather than crashing.
func openLogFile() *os.File {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".cloudtui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, "cloudtui.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil
	}
	return f
}
