package ui

import (
	"log/slog"

	"github.com/gen2brain/beeep"
)

// DesktopNotify sends an OS-level desktop notification — macOS via
// osascript/terminal-notifier, Linux via D-Bus with a notify-send
// fallback, Windows via the WinRT COM API with a PowerShell fallback
// (see spec/43-fe-codepipeline-monitor for why beeep). A failure (e.g.
// headless Linux with no D-Bus session) is logged and otherwise
// swallowed — desktop notifications are a nice-to-have, never fatal,
// and must never block whatever background work triggered them.
func DesktopNotify(title, message string) {
	if err := beeep.Notify(title, message, ""); err != nil {
		slog.Warn("desktop notification failed", "title", title, "error", err)
	}
}
