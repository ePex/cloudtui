# FE: split config.yaml into settings/connections/favorites files

Date: 2026-09-04

## Purpose

`~/.cloudtui/config.yaml` today holds everything: appearance settings
(theme, colors, logo), the `activeAWSProfile` pointer, Datadog
settings, the full `connections:` list (both Jolokia and proxy
backends mixed together), and `awsFavorites:` (starred SSM
parameters/secrets/log groups). As a user accumulates connections and
favorites, this one file grows into an awkward mix of "things I rarely
touch" (appearance) and "things that grow over time and might be worth
handling separately" (connections, favorites) — `config.example.yaml`
already hints at this tension today, noting favorites are shown "for
reference (e.g. if you want to pre-seed or copy favorites between
machines)," which currently means manually extracting just that one
YAML key out of a file that also has connection passwords in it.

Confirmed scope for this request (via user Q&A, 2026-09-04): both the
favorites split and the connections split are **file organization
only** — no new sharing/sync mechanism, no personal-vs-team merge
logic. "Shareable" means "its own file, so a user can manually copy or
commit *just* that file without dragging along everything else" — the
app itself doesn't gain any new capability to load from an alternate
path or merge multiple sources.

## Scope

- Split the single `config.yaml` into three files under
  `~/.cloudtui/`:
  1. **Settings** — stays at `config.yaml`: `theme`, `logo`, `colors`,
     `activeAWSProfile`, `datadog`, `activeConnection` (the *pointer*
     to which connection is active — stays with settings since it's a
     "current selection," the same category as `activeAWSProfile`;
     the connection *data* itself moves out).
  2. **Connections, split by backend type** — new location(s), exact
     path decided in `plan.md`: Jolokia connections and proxy
     connections in separate files. The `Connection` struct/schema is
     unchanged (including its existing `backend` field) — this is a
     storage-location split, not a data-model change, per the
     "organization only" scope confirmed above.
  3. **Favorites** — new location, exact path decided in `plan.md`:
     `awsFavorites`' content, standalone.
- **One-time migration**: a pre-existing single-file `config.yaml`
  (today's format) is split automatically on first load after this
  ships — same "detect legacy content, migrate in place" pattern
  `Load()` already uses twice today (the pre-relocation `tui/config.yaml`
  → `~/.cloudtui/config.yaml` move, and the pre-FE22 top-level
  `backend`/`queue`/`proxy` → `connections:` migration). Nothing is
  lost or requires manual user action.
- **In-memory `Config` struct is unchanged.** `Connections
  []Connection`, `AWSFavorites`, `ActiveConnection`, etc. all stay
  exactly as they are today, read by the rest of the app exactly as
  today (`a.cfg.Connections`, `a.cfg.AWSFavorites`, ...) — confirmed
  via a full grep of every call site that touches these fields or
  calls `config.Save`/`SaveDefault`. Every mutation already goes
  through one `config.SaveDefault(a.cfg)` call
  (`internal/app/host.go`'s `SaveConnection`/`DeleteConnection`/
  `ToggleFavorite`), so this refactor is confined to `Load`/`Save`/
  `LoadDefault`/`SaveDefault`/`DefaultPath` in
  `internal/config/config.go` — nothing in `internal/app`,
  `internal/dialog`, or `internal/view` needs to change.
- `tui/config.example.yaml` updated to reflect the new file layout
  (exact shape — one example file per real file, or one doc explaining
  the split — decided in `plan.md`).
- Merge-back updates the ~10 spec files that currently describe the
  single-file format (`spec/01-repo-and-tui-shell`,
  `spec/12-named-connections`, and others found via grep) to describe
  the new layout.

## Out of scope

- **No new sharing/sync mechanism** — confirmed via Q&A. No
  alternate-path loading, no env var pointing at a shared favorites
  location, no personal+team merge. A user who wants to share
  favorites or connections copies the file themselves, same as today's
  `config.example.yaml` already suggests for favorites.
- **No change to the `Connection`/`AWSFavorites`/`Palette` schemas
  themselves** — field names, types, and YAML tags are unchanged; only
  which file each top-level section lives in changes.
- **No change to secret resolution, password handling, or any
  connect-time behavior** — purely a config-storage reorganization.
- **No change to the Settings UI** — it already edits fields via
  modal dialogs that call `Host.SaveConnection`/`DeleteConnection`/
  `ToggleFavorite`/etc., all of which stay untouched (see above).

## Data & config

New file(s) under `~/.cloudtui/` (exact names in `plan.md`) for
connections and favorites; `config.yaml` stays for settings. Touches
`tui/internal/config/config.go` (and its `_test.go`),
`tui/config.example.yaml`, and the ~10 `spec/` files referencing the
old single-file format.
