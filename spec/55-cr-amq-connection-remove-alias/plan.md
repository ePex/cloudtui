# Plan — CR 55: Drop the connection `alias` field

## Approach

Delete `Connection.Alias` and every read/write of it, then reindex the two
places that reference form-item positions by numeric index
(`connEditorForm`'s `GetFormItem(N)` calls, since removing one `AddInputField`
shifts everything after it down by one). No migration code: `yaml.Unmarshal`
already ignores unknown keys by default (this is exactly how the pre-FE22
legacy `backend`/`queue`/`proxy` fields get ignored once a file has
`connections`), so a leftover `alias:` key in someone's local `config.yaml`
just stops being read — `Save()` won't write it back.

No new dependencies, no config-schema migration, no backwards-compat shim
needed — this is a pure deletion plus a handful of `.Alias` → `.Name`
swaps.

## Files touched

1. **`tui/internal/config/config.go`**
   - Remove `Alias string` from `Connection`.
   - `Default()`: drop `Alias: "def"` from the seeded connection.

2. **`tui/internal/config/config_test.go`**
   - `TestDefaultConnectionAlias` (line ~270): delete — nothing left to
     assert.
   - The YAML-round-trip test around line ~295 that sets `alias: stg` in
     the fixture and asserts `conn.Alias == "stg"` (line ~311): drop the
     `alias:` line from the fixture YAML and the assertion; keep the rest of
     the round-trip coverage (name/backend/proxy fields) intact. Add one
     assertion that a fixture *with* a stale `alias:` key still loads
     without error and produces a `Connection` with the right `Name` — covers
     the "silently ignored" claim in spec.md explicitly, rather than leaving
     it implicit.

3. **`tui/internal/app/topbar.go`**
   - `infoPanelText`: `cfg.ActiveConn().Alias` → `cfg.ActiveConn().Name`.

4. **`tui/internal/app/topbar_test.go`**
   - `TestInfoPanelContainsConnectionAlias` and
     `TestInfoPanelTextShowsConnectionAlias`: rename to
     `...ConnectionName`, assert against `Name` instead of `Alias` (set
     `cfg.Connections[0].Name = "staging"` instead of `.Alias = "stg"`).

5. **`tui/internal/app/settings.go`**
   - `refreshSettingsList`: `conn.Alias` → `conn.Name` in the
     `"AMQ Connection: %s"` item text; update the doc comment above it
     (currently says "active connection alias").

6. **`tui/internal/app/connections.go`**
   - `populateConnManagerList`: label format `"%s%-8s %-24s (%s)"` (star,
     alias, name, backend) → `"%s%-24s (%s)"` (star, name, backend).
   - `showConnEditor`: remove the `GetFormItem(1)` Alias prefill line;
     `GetFormItem(2..6)` (Backend..Password) become `GetFormItem(1..5)`.
   - `saveConnEditor`: remove the `alias :=` read (old `GetFormItem(1)`);
     shift `GetFormItem(2..6)` reads to `GetFormItem(1..5)`; drop `alias`
     from the `"Name and alias are required"` validation and the
     `config.Connection{Name: name, Alias: alias, ...}` literal — becomes
     `config.Connection{Name: name, ...}`.

7. **`tui/internal/app/app.go`**
   - Form construction (~line 480): delete the
     `.AddInputField("Alias", "", 10, nil, nil)` line.
   - `styleDropDown(dd, cfg.Colors)` lookup: `GetFormItem(2)` → `GetFormItem(1)`
     (Backend dropdown is now the second item).
   - Duplicate handler (`'d'` case, ~line 447-454): delete the
     `al := dup.Alias + "2"` / truncate-to-8-runes / `dup.Alias = al` block;
     keep only `dup.Name = dup.Name + "-copy"`.
   - Height comment + `centered(a.connEditorForm, 64, 20)` call: recompute
     for 6 form items instead of 7 (border+padding 4 rows + 6×2=12 rows +
     button row 1 row = 17, +1 spare = 18). Update the comment and the
     literal `20` → `18`; confirm the actual rendered size looks right
     during manual verification (tview's exact per-item row count has been
     wrong-by-one in this codebase before, per the comment already there).

8. **`tui/internal/devtool/config.go`**
   - `AddProxyConnection(cfg, name, alias, url, username, password)` →
     `AddProxyConnection(cfg, name, url, username, password)`; drop
     `Alias: alias` from the appended `config.Connection{}`.

9. **`tui/internal/devtool/config_test.go`**
   - Update the two `{Name: "default", Alias: "def", ...}` fixtures to drop
     `Alias`.
   - Update the call to `AddProxyConnection` to the new 5-arg signature and
     the assertion (`added.Name != "smoke" || added.Alias != "smk"` → just
     `added.Name != "smoke"`).

10. **`tui/cmd/devtool/main.go`**
    - Usage string: `add-proxy-conn <name> <alias> <url> <username>
      <password>` → `add-proxy-conn <name> <url> <username> <password>`.
    - `add-proxy-conn` case: `len(os.Args) != 7` → `!= 6`; call
      `devtool.AddProxyConnection(cfg, os.Args[2], os.Args[3], os.Args[4],
      os.Args[5])` (drops the old `os.Args[3]` alias, shifts url/username/
      password down one); success message drops `(alias %q)`.

11. **`tui/config.example.yaml`**
    - Remove the `alias:` line from each example connection entry.

12. **`spec/22-fe-connections/spec.md`**
    - Update the "Config shape" YAML block to drop `alias:` lines.
    - Amend the paragraph explaining `alias` (currently: "`alias` is a short
      label... `name` is the human-readable identifier...") to note the
      field was removed by CR 55, with a link, rather than deleting the
      historical explanation outright — spec docs are a record of what was
      built, and FE 22 genuinely shipped with an alias field.
    - Update the "Connection manager overlay" bullet (`Lists all connections
      as <alias> <name> (<backend>)`) and the "Connection editor overlay"
      bullet (form field list) to match the new shape.
    - Leave the "Files touched" table and "Definition of done" as historical
      record — not amending those retroactively.

`tui/config.yaml` itself is gitignored/local, not touched by this change —
existing local files just lose their `alias:` key silently on next save, as
described in spec.md.

## Testing

- Existing unit tests updated per items 2, 4, 9 above; no new test files.
- `go build ./...` and `go test ./...` in `tui/`.
- Manual: run the app, open Settings → AMQ Connection, confirm the manager
  list, editor form (no Alias field, Tab order Name→Backend→...), duplicate,
  and info panel all show `name` with no trace of `alias`. Confirm the
  editor overlay still renders without clipping at its new height.
