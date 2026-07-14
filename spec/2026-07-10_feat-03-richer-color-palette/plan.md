# Plan — Richer color palette

Spec: [spec.md](spec.md)

## Approach

### `Palette` schema (`internal/config`)

```go
type Palette struct {
    Border  string            `yaml:"border"`
    Label   string            `yaml:"label"`
    Value   string            `yaml:"value"`
    Accent  string            `yaml:"accent"`
    Success string            `yaml:"success"`
    Warning string            `yaml:"warning"`
    Error   string            `yaml:"error"`
    Views   map[string]string `yaml:"views"`
}

// ViewColor returns the configured color for the named view, falling
// back to Border if the view isn't listed — so a view added later
// without a palette update still gets a sensible border color.
func (p Palette) ViewColor(name string) string {
    if c, ok := p.Views[name]; ok && c != "" {
        return c
    }
    return p.Border
}
```

`Default()` adds `Success: "green"`, `Warning: "yellow"`, `Error: "red"`,
and a `Views` map with an entry for each of the five current views
(`home`, `secrets`, `params`, `queues`, `settings`), each a distinct
color.

### Applying colors to resource views (`internal/app`)

Rather than changing `internal/ui/views`' constructor signatures (which
would ripple through all five constructors and their tests for something
that's really shell-level styling), `App.New()`'s existing
view-registration loop applies the color externally, via a small
structural interface satisfied by anything embedding `*tview.Box`
(which is every current placeholder, and will be any future real view
too):

```go
// bordered is implemented by tview primitives (via an embedded
// *tview.Box) that expose settable border/title colors.
type bordered interface {
    SetBorderColor(color tcell.Color) *tview.Box
    SetTitleColor(color tcell.Color) *tview.Box
}

for _, v := range a.views {
    prim := v.Primitive()
    if b, ok := prim.(bordered); ok {
        c := tcell.GetColor(cfg.Colors.ViewColor(v.Name()))
        b.SetBorderColor(c)
        b.SetTitleColor(c)
    }
    a.pages.AddPage(v.Name(), prim, true, false)
}
```

This keeps `internal/ui/views` free of any `internal/config` dependency;
`internal/app` (which already depends on both) does the wiring.

### `config.example.yaml`

Document `success`/`warning`/`error` and a `views:` map under `colors:`.

## Files touched

- `tui/internal/config/config.go` (modified) — `Palette` fields +
  `ViewColor`; `Default()` updated.
- `tui/internal/config/config_test.go` (modified) — updated `TestDefault`
  expectations; new tests for `ViewColor` fallback and for partial
  `colors.views`/new-scalar-field merge behavior.
- `tui/config.example.yaml` (modified) — document the new fields.
- `tui/internal/app/app.go` (modified) — `bordered` interface + coloring
  in the view-registration loop.
- `tui/internal/app/app_test.go` (modified) — assert registered views'
  rendered border color matches their configured (or fallback) color.

No new dependency; no changes to `internal/ui/views` or its tests.

## Key decisions / trade-offs

- **Border/title coloring lives in `internal/app`, not
  `internal/ui/views`**, via the `bordered` structural interface — avoids
  coupling the views package to config and avoids touching all five
  view constructors for a shell-styling concern.
- **`Views` is a plain `map[string]string` keyed by view name**, matching
  the existing string-based `ui.View.Name()`/`switchTo` convention
  everywhere else in the app, rather than a stricter enum — an unlisted
  name just falls back to `Border`.
- **`Success`/`Warning`/`Error` are schema-only this pass** (per the
  approved spec) — added with a short comment noting they're
  intentionally unused yet, so they don't read as dead code later.
- **Map-merge behavior for partial `colors.views` overrides** is expected
  to fall out of `yaml.v3`'s map decoding (reusing the pre-populated
  target map and adding/overwriting only the keys present in the
  document, mirroring `encoding/json`'s documented map-decode semantics)
  — the same `Load()` code as before (`cfg := Default(); yaml.Unmarshal(data,
  &cfg)`) should just work. This is verified with a dedicated test rather
  than assumed; if the test shows the whole map gets replaced instead,
  `Load` gains an explicit post-unmarshal merge step for `Views` as a
  fallback, decided from what the test actually shows.
- **Only `GetBorderColor()` is asserted in tests**, not title color —
  `tview.Box` has a border-color getter but no title-color getter, so
  `SetTitleColor` is still called (for visual consistency) but isn't
  independently verifiable.

## Testing

- `internal/config`: `Default()` includes the three status colors and
  the five-entry `Views` map; `Palette.ViewColor` returns the mapped
  color when present and `Border` otherwise; `Load()` with a config that
  only overrides one `colors.views` entry (or one new scalar field like
  `warning`) leaves the rest at their defaults.
- `internal/app`: after `New()`, at least one default-mapped view (e.g.
  `"secrets"`) renders with its configured `GetBorderColor()`, and a view
  not present in the configured `Views` map (a fake view appended in the
  test, similar to the existing `fakeFilterableView` pattern) falls back
  to `Border`'s color.
