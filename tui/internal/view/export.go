package view

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/export"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// exportSet is what an export writes, and whether that's everything:
// total > len(msgs) or more means the export is incomplete, why says why.
type exportSet struct {
	msgs    []queue.Message
	total   int64 // the queue's pending count when known and higher; 0 otherwise
	hasMore bool  // the backend reported more beyond msgs
	why     string
}

// exportFlow is the export action shared by the queue list and the message
// browser (spec-wip fe-message-export): path prompt → overwrite confirm →
// collect → write → status line.
type exportFlow struct {
	host    ui.Host
	prompt  *dialog.TextPrompt
	confirm *dialog.ConfirmDialog
	now     func() time.Time
}

func newExportFlow(host ui.Host, prompt *dialog.TextPrompt, confirm *dialog.ConfirmDialog) exportFlow {
	return exportFlow{host: host, prompt: prompt, confirm: confirm, now: time.Now}
}

// defaultPath is the path the prompt is prefilled with.
func (ef exportFlow) defaultPath(queueName string) string {
	return filepath.Join(export.DefaultDir(), export.DefaultFileName(queueName, ef.now()))
}

// start asks where to export queueName to. collect gathers the messages on
// a background goroutine once a path is settled. onClose restores the
// calling view's focus whenever a dialog of the flow closes.
func (ef exportFlow) start(queueName string, collect func(context.Context) (exportSet, error), onClose func()) {
	ef.askPath(queueName, ef.defaultPath(queueName), collect, onClose)
}

func (ef exportFlow) askPath(queueName, initial string, collect func(context.Context) (exportSet, error), onClose func()) {
	var chosen string
	ef.prompt.Show(fmt.Sprintf("Export %q", queueName), "File:", initial, func(text string) error {
		path, err := validateExportPath(text)
		if err != nil {
			return err
		}
		chosen = path
		return nil
	}, func() {
		onClose()
		// Continue only once the prompt has closed and handed focus back —
		// a confirm opened from onSubmit would lose focus to onClose.
		if chosen != "" {
			ef.proceed(queueName, chosen, collect, onClose)
		}
	})
}

// validateExportPath trims path and rejects an empty one or one whose
// folder doesn't exist; the prompt stays open with the error.
func validateExportPath(text string) (string, error) {
	path := strings.TrimSpace(text)
	if path == "" {
		return "", errors.New("enter a file path")
	}
	dir := filepath.Dir(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder %s doesn't exist", dir)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return "", fmt.Errorf("%s is a folder", path)
	}
	return path, nil
}

// proceed confirms overwriting an existing file (No returns to the path
// prompt), then runs the export.
func (ef exportFlow) proceed(queueName, path string, collect func(context.Context) (exportSet, error), onClose func()) {
	if _, err := os.Stat(path); err != nil {
		ef.run(queueName, path, collect)
		return
	}
	// Question first and just the file name: the confirm shows three rows,
	// a long path would push the question out of sight.
	ef.confirm.ShowWithCancel(fmt.Sprintf("Overwrite the existing file %s?", filepath.Base(path)), func() {
		onClose()
		ef.run(queueName, path, collect)
	}, func() {
		ef.askPath(queueName, path, collect, onClose)
	})
}

// run collects and writes on a background goroutine; the result lands in
// the status line via finish.
func (ef exportFlow) run(queueName, path string, collect func(context.Context) (exportSet, error)) {
	ef.host.SetStatus(fmt.Sprintf("Exporting %q…", queueName))
	go func() {
		set, err := collect(context.Background())
		if err == nil {
			err = export.WriteFile(path, queueName, set.msgs)
		}
		ef.host.QueueUpdateDraw(func() { ef.finish(queueName, path, set, err) })
	}()
}

// finish reports a completed export — split out from run's goroutine so
// tests can call it directly, like purgeFlow.finish.
func (ef exportFlow) finish(queueName, path string, set exportSet, err error) {
	if err != nil {
		slog.Error("export failed", "queue", queueName, "path", path, "error", err)
		ef.host.SetStatus(fmt.Sprintf("[red]Export failed: %s[-]", tview.Escape(err.Error())))
		return
	}
	ef.host.SetStatus(tview.Escape(exportedStatus(path, set)))
}

// exportedStatus is the result line: "Exported <n> message(s) to <path>",
// saying so when the export couldn't be complete. The note comes before
// the path: a long path runs off the status line's end, the note mustn't.
func exportedStatus(path string, set exportSet) string {
	n := len(set.msgs)
	switch {
	case set.total > int64(n):
		return fmt.Sprintf("Exported %d of %d message(s) (%s) to %s", n, set.total, set.why, path)
	case set.hasMore:
		return fmt.Sprintf("Exported %d message(s), more on the queue (%s), to %s", n, set.why, path)
	default:
		return fmt.Sprintf("Exported %d message(s) to %s", n, path)
	}
}

// collectQueue browses all of queueName for an export (up to
// export.MaxMessages) plus its pending count, to tell when the broker's
// browse limit cut the export short.
func collectQueue(b queue.Backend, queueName string) func(context.Context) (exportSet, error) {
	return func(ctx context.Context) (exportSet, error) {
		msgs, hasMore, err := b.BrowseMessages(ctx, queueName, queue.MessageFilter{MaxCount: export.MaxMessages})
		if err != nil {
			return exportSet{}, err
		}
		set := exportSet{msgs: msgs, hasMore: hasMore, why: "the broker's browse limit"}
		if pending, ok := pendingCount(ctx, b, queueName); ok && pending > int64(len(msgs)) {
			set.total = pending
		}
		return set, nil
	}
}
