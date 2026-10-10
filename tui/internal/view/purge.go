package view

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// purgeFlow is the purge action shared by the queue list and the message
// browser (spec/09): JMS Type prompt → confirm → backend call → status
// line. Both views' `p` go through it, so the two entry points can't
// drift apart again.
type purgeFlow struct {
	host    ui.Host
	backend func() queue.Backend // read at purge time: the active connection can change
	prompt  *dialog.JMSTypePrompt
	confirm *dialog.ConfirmDialog
}

// start opens the JMS Type prompt for queueName. onClose runs whenever the
// prompt closes (Enter or Esc), as for JMSTypePrompt.Show. After a
// confirmed purge, on the UI goroutine: onDone on success or a stop
// (after the status line is set), onError on failure. The purge runs as
// a stoppable operation in the progress modal; Esc stops it before its
// next request (spec/24).
func (pf purgeFlow) start(queueName string, onClose, onDone func(), onError func(error)) {
	pf.prompt.Show("Purge", queueName, func(jmsType string) {
		pf.confirm.Show(purgeQuestion(queueName, jmsType), func() {
			op := pf.host.StartOperation(purgeTitle(queueName, jmsType), 0, ui.OpStoppable)
			go func() {
				b := pf.backend()
				n, err := purge(op.Ctx(), b, queueName, jmsType)
				err = unlessQueueEmptied(b, queueName, jmsType, err)
				var warning string
				if err == nil {
					warning = filteredRemainderWarning(context.Background(), b, queueName, jmsType)
				}
				pf.host.QueueUpdateDraw(func() {
					op.Finish()
					pf.finish(queueName, jmsType, n, warning, err, onDone, onError)
				})
			}()
		})
	}, onClose)
}

// purgeTitle is the progress modal's title for a purge.
func purgeTitle(queueName, jmsType string) string {
	if jmsType == "" {
		return fmt.Sprintf("Purging %q", queueName)
	}
	return fmt.Sprintf("Purging %s messages from %q", jmsType, queueName)
}

// cancelledSuffix ends the status line of an operation the user stopped
// with Esc before it was done.
const cancelledSuffix = " (cancelled)"

// unlessQueueEmptied clears err when it's a stop (context.Canceled) of an
// unfiltered purge or move-all that had already emptied queueName: its
// last request removed everything and the stop only skipped the check
// for more, so nothing was cancelled. A filtered one keeps the stop — the
// queue's total can't tell whether matching messages are left — as does
// one whose pending count can't be read.
func unlessQueueEmptied(b queue.Backend, queueName, jmsType string, err error) error {
	if !errors.Is(err, context.Canceled) || jmsType != "" {
		return err
	}
	if n, ok := pendingCount(context.Background(), b, queueName); ok && n == 0 {
		return nil
	}
	return err
}

// finish applies a completed purge's outcome — split out from start's
// goroutine so tests can call it directly, like the dialogs'
// handleScanResult. warning (from filteredRemainderWarning) is appended to
// the status line. A purge stopped with Esc isn't an error: it reports
// what it removed so far, marked as cancelled.
func (pf purgeFlow) finish(queueName, jmsType string, n int, warning string, err error, onDone func(), onError func(error)) {
	if errors.Is(err, context.Canceled) {
		pf.host.SetStatus(purgedStatus(queueName, jmsType, n) + cancelledSuffix)
		onDone()
		return
	}
	if err != nil {
		slog.Error("purge failed", "queue", queueName, "jmsType", jmsType, "deleted", n, "error", err)
		onError(err)
		return
	}
	pf.host.SetStatus(purgedStatus(queueName, jmsType, n) + warning)
	onDone()
}

// filteredWindowWarnAt is ActiveMQ's default maxPageSize: a consumer with a
// JMS selector only sees matches among the first this-many messages the
// broker pages in, so a filtered purge/move-all can miss matches behind
// more non-matching messages than that (spec-wip bugfix-purge-completeness,
// "What JMS can and can't reach"). A heuristic: a broker configured with a
// larger window makes the warning show more often than needed.
const filteredWindowWarnAt = 200

// filteredRemainderWarning is the status-line suffix after a filtered
// (jmsType != "") purge or move-all: when more than filteredWindowWarnAt
// messages remain on queueName, matching ones further back may not have
// been reached. "May": the TUI can't tell whether the remaining messages
// include matches. Empty for an unfiltered operation, at or below the
// threshold, or when the pending count can't be read (logged, not an
// error — the operation itself succeeded).
func filteredRemainderWarning(ctx context.Context, b queue.Backend, queueName, jmsType string) string {
	if jmsType == "" {
		return ""
	}
	if n, ok := pendingCount(ctx, b, queueName); ok && n > filteredWindowWarnAt {
		return fmt.Sprintf(" — %d message(s) remain; matching messages further back may not have been reached", n)
	}
	return ""
}

// pendingCount reads queueName's pending count via List. ok is false when
// List fails (logged) or doesn't include the queue.
func pendingCount(ctx context.Context, b queue.Backend, queueName string) (int64, bool) {
	summaries, err := b.List(ctx)
	if err != nil {
		slog.Warn("can't read the queue's pending count", "queue", queueName, "error", err)
		return 0, false
	}
	for _, s := range summaries {
		if s.Name == queueName {
			return s.PendingCount, true
		}
	}
	return 0, false
}

// purge dispatches to the backend call matching jmsType: PurgeQueue (the
// existing fast, native path) when empty, or DeleteMessages with a JMSType
// filter otherwise — see spec/09's "Preserving the existing unfiltered
// path" for why these stay two different backend calls rather than
// DeleteMessages with an always-empty-filter fallback.
func purge(ctx context.Context, b queue.Backend, queueName, jmsType string) (int, error) {
	if jmsType == "" {
		return b.PurgeQueue(ctx, queueName)
	}
	return b.DeleteMessages(ctx, queueName, queue.MessageFilter{JMSType: jmsType})
}

// purgeQuestion is the confirm dialog's question, worded by jmsType
// (empty = every message).
func purgeQuestion(queueName, jmsType string) string {
	if jmsType == "" {
		return fmt.Sprintf("Purge %q? All messages will be deleted.", queueName)
	}
	return fmt.Sprintf("Purge %q? All %s messages will be deleted.", queueName, jmsType)
}

// purgedStatus is the status line after a successful purge.
func purgedStatus(queueName, jmsType string, n int) string {
	if jmsType == "" {
		return fmt.Sprintf("Purged %d message(s) from %q", n, queueName)
	}
	return fmt.Sprintf("Purged %d %s message(s) from %q", n, jmsType, queueName)
}
