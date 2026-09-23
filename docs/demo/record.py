#!/usr/bin/env python3
"""Records the README demo GIFs on the example instance (see README.md here).

Each scenario drives the real TUI in a tmux session, samples the screen
(with colors) about ten times a second, writes the frames as an asciinema
recording, and renders that to a GIF with agg. No browser or network is
involved, so it also works in sandboxed shells.

Usage (from the repo root, after docs/demo/setup.sh):
    docs/demo/record.py            # all scenarios, in order
    docs/demo/record.py queues     # just one (they build on each other's
                                   # data, so a fresh setup + all is safest)
Needs tmux and agg. Standard library only.
"""
import json
import os
import subprocess
import sys
import threading
import time

DEMO = "/tmp/cloudtui-demo"
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)))
SESSION = "cloudtui-demo"
WIDTH, HEIGHT = 150, 40
SAMPLE_EVERY = 0.1  # seconds
TYPE_DELAY = 0.07   # seconds per typed character


def tmux(*args):
    return subprocess.run(["tmux", *args], check=True, capture_output=True, text=True).stdout


# ── Scenario steps ────────────────────────────────────────────────────────
# ("key", name[, pause]) presses a tmux key name (Enter, Down, Escape, BSpace,
# BTab, Tab, a single character, ...); ("type", text[, pause]) types text
# one character at a time; ("paste", text[, pause]) sends it all at once,
# so no in-between screen (e.g. an autocomplete list) is recorded;
# ("wait", seconds) just lets the screen sit.

def key(name, pause=0.6):
    return ("key", name, pause)


def type_(text, pause=0.6):
    return ("type", text, pause)


def paste(text, pause=0.6):
    return ("paste", text, pause)


def wait(seconds):
    return ("wait", seconds)


def keys(*names, pause=0.25):
    return [key(n, pause) for n in names]


def prompt(command):
    """Switch view with the ':' command prompt."""
    return [key(":", 0.4), type_(command, 0.5), key("Enter", 0.3), key("Enter", 1.8)]


SCENARIOS = {
    # Home → queue list → filter → messages → message detail.
    "queues": [
        wait(1.5),
        key("Enter", 2.0),
        key("/", 0.4), type_("orders", 0.8), key("Enter", 1.0),
        key("Down", 0.8),                        # past dlq.orders.created
        key("Enter", 2.0),                       # orders.created's messages
        key("Down", 0.5), key("Down", 0.8),
        key("Enter", 3.5),
        key("Escape", 1.0), key("Escape", 1.5),
    ],
    # Compose a message on a queue from a library snippet and send it.
    "send-snippet": [
        *prompt("queues"),
        key("/", 0.4), type_("orders.created", 0.6), key("Enter", 1.0),
        key("Down", 0.8),                       # past dlq.orders.created
        key("c", 1.2),
        *keys("Tab", "Tab", "Tab", "Tab", "Tab", "Tab", "Tab", pause=0.15),
        key("Enter", 1.2),                      # Load snippet…
        key("Down", 0.6), key("Enter", 0.8),    # orders/
        key("Down", 0.4), key("Down", 0.6),     # order-created.json
        key("Enter", 1.8),
        key("BTab", 0.2), key("BTab", 0.5), key("Enter", 2.0),  # Submit
        key("Enter", 2.5),                      # the queue's messages
    ],
    # The snippet library: browse with preview, then import a plain JSON
    # file — the editor opens on JMS Type for it.
    "snippets": [
        *prompt("snippets"),
        key("Down", 1.2),                       # orders/: its counts
        key("Enter", 0.8), key("Down", 1.5), key("Down", 1.8),  # JSON previews
        key("BSpace", 0.6), key("Down", 0.8),   # payments/
        key("Enter", 0.8), key("Down", 2.0),    # XML preview
        key("BSpace", 1.0),
        key("i", 0.8), type_(f"{DEMO}/files/order-ORD-1042.json", 0.6), key("Enter", 1.2),
        key("Enter", 2.0),                      # accept the suggested name
        type_("OrderCreated", 0.6), key("Enter", 2.5),
    ],
    # Requeue a whole DLQ: mark all, move, and the matching queue is offered first.
    "dlq-requeue": [
        *prompt("queues"),
        key("/", 0.4), type_("dlq", 0.6), key("Enter", 1.0),
        key("Enter", 2.0),                       # the DLQ's messages
        key("a", 1.0),                           # mark all
        key("m", 2.0),                           # move: orders.created is offered first
        key("Enter", 2.5),
        key("Escape", 1.0),                      # back to the queue list
        key("/", 0.4), key("C-u", 0.3), key("Enter", 0.3),  # clear the filter
        key("r", 2.5),
    ],
    # Live theme switching with :theme (dark → cyberpunk → dark). Pasted
    # whole so the autocomplete never lists every bundled theme on screen.
    "themes": [
        *prompt("queues"),
        wait(1.0),
        key(":", 0.4), paste("theme cyberpunk", 0.8), key("Enter", 0.3), key("Enter", 2.5),
        key("Down", 0.4), key("Down", 1.0),
        key(":", 0.4), paste("theme dark", 0.8), key("Enter", 0.3), key("Enter", 2.5),
    ],
}


class Recorder:
    """Samples the tmux pane into asciicast v2 output events."""

    def __init__(self):
        self.events = []
        self.last = None
        self.start = time.monotonic()
        self.stop = threading.Event()
        self.thread = threading.Thread(target=self.loop, daemon=True)

    def frame(self):
        rows = tmux("capture-pane", "-t", SESSION, "-p", "-e").split("\n")[:HEIGHT]
        # Reset attributes at each row end so backgrounds never bleed.
        return "\x1b[?25l\x1b[H\x1b[2J" + "\r\n".join(r + "\x1b[0m" for r in rows)

    def sample(self):
        f = self.frame()
        if f != self.last:
            self.events.append([round(time.monotonic() - self.start, 3), "o", f])
            self.last = f

    def loop(self):
        while not self.stop.is_set():
            self.sample()
            time.sleep(SAMPLE_EVERY)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *exc):
        self.stop.set()
        self.thread.join()
        self.sample()


def run(name, steps):
    env_home = f"{DEMO}/home"
    subprocess.run(["tmux", "kill-session", "-t", SESSION], capture_output=True)
    tmux("new-session", "-d", "-s", SESSION, "-x", str(WIDTH), "-y", str(HEIGHT), "-c", DEMO,
         f"HOME={env_home} {DEMO}/bin/cloudtui")
    time.sleep(2.5)  # startup isn't recorded

    with Recorder() as rec:
        for step in steps:
            kind = step[0]
            if kind == "wait":
                time.sleep(step[1])
            elif kind == "key":
                tmux("send-keys", "-t", SESSION, step[1])
                time.sleep(step[2])
            elif kind == "paste":
                tmux("send-keys", "-t", SESSION, "-l", step[1])
                time.sleep(step[2])
            elif kind == "type":
                for ch in step[1]:
                    tmux("send-keys", "-t", SESSION, "-l", ch)
                    time.sleep(TYPE_DELAY)
                time.sleep(step[2])
        time.sleep(1.0)

    tmux("send-keys", "-t", SESSION, "q")
    time.sleep(0.5)
    subprocess.run(["tmux", "kill-session", "-t", SESSION], capture_output=True)

    cast = os.path.join(DEMO, f"{name}.cast")
    with open(cast, "w") as f:
        f.write(json.dumps({"version": 2, "width": WIDTH, "height": HEIGHT,
                            "env": {"TERM": "xterm-256color"}}) + "\n")
        for event in rec.events:
            f.write(json.dumps(event) + "\n")

    gif = os.path.join(OUT, f"{name}.gif")
    subprocess.run(["agg", "--font-size", "13", "--fps-cap", "12", "--idle-time-limit", "3",
                    "--last-frame-duration", "2.5", cast, gif], check=True, capture_output=True)
    print(f"{gif}: {len(rec.events)} frames, {os.path.getsize(gif) // 1024} KiB")


def main():
    names = sys.argv[1:] or list(SCENARIOS)
    for name in names:
        run(name, SCENARIOS[name])


if __name__ == "__main__":
    main()
