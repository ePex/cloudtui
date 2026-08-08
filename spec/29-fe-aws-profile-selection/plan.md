# Plan — FE 29: select an active AWS profile, filter the list, `:ap`

## `config.Config`

```go
ActiveAWSProfile string `yaml:"activeAWSProfile"`
```

Top-level, alongside `ActiveConnection` — not nested under `Connection`,
per this spec's stated (partial, deliberately non-final) answer to FE 28's
open question.

## `topbar.go`

`infoPanelText` gains a third line; `cfg.ActiveAWSProfile` empty renders as
`(none)`.

## `awsprofiles.go` / `app.go`

New fields: `awsProfilesFilterInput *tview.InputField`,
`awsProfilesFilter string`, `awsProfilesAll`/`awsProfilesFiltered
[]awsprofile.Profile` (full vs. currently-displayed, mirroring
`queuesView`'s `allSummaries`/filtered-in-`repaint` split).

- `populateAWSProfilesTable`: re-runs `a.listAWSProfiles`, stores into
  `awsProfilesAll`, calls `repaintAWSProfiles`.
- `repaintAWSProfiles`: filters `awsProfilesAll` by
  `awsProfilesFilter` (case-insensitive substring on name) into
  `awsProfilesFiltered`, redraws rows from that, marks the active profile
  with `⭐ ` prefix + accent color, sets the title to
  `" AWS Profiles [filter] "` or `" AWS Profiles (N) "`.
- `applyAWSProfilesFilter`: updates the filter and re-repaints — no disk
  I/O, unlike `populateAWSProfilesTable`.
- `activateAWSProfile(name)`: sets `cfg.ActiveAWSProfile`, updates the info
  panel, closes the overlay, sets a status-bar message, persists via
  `config.SaveDefault`.
- Table `SetSelectedFunc` maps the selected row to
  `awsProfilesFiltered[row-1]` (not `awsProfilesAll`) — critical under an
  active filter, same reasoning as `messagesView.msgs` being the
  currently-displayed (sorted/filtered) snapshot that row indices map into.
- `showAWSProfiles` resets `awsProfilesFilter`/`awsProfilesFilterInput`
  text on every open — a stale filter from a previous visit would be
  confusing to land on.
- Filter input wiring (`/` to focus, `SetChangedFunc`/`SetDoneFunc`,
  Up/Down redirect back to the table while focused) copied from
  `queuesView`'s exact pattern.

## `onPromptDone`

```go
case cmd == "ap" || cmd == "awsprofiles":
    a.showAWSProfiles()
```

No new focus-guard work needed — FE 27's guard already included
`awsProfilesVisible` when it was added (pre-emptively, before this command
existed).

## Testing

`config`: round-trip test for `ActiveAWSProfile`; default-empty test.

`topbar`: `(none)` when unset; shows the name when set.

`app`/`awsprofiles`: filter narrows rows and updates the title; clearing
the filter restores the full list; reopening resets a stale filter;
activation persists + updates the info panel + closes + writes
`config.yaml` (in a `t.Chdir(t.TempDir())` sandbox); the active profile is
starred and others aren't; `Enter` activates the *filtered* row, not the
unfiltered index (this is the one most likely to silently regress and
activate the wrong profile).

`app`: `:ap`/`:awsprofiles` open the overlay with correct focus, from a
non-Settings view.

## No new dependencies
