# CR 05 — Home view: navigatable sections

Date: 2026-08-05

## What and why

The current home view is a flat, non-interactive table listing all registered
views. It has two problems:

1. **Not navigatable.** The user can only switch views via hotkeys or the
   command prompt. There is no way to use the home screen as an actual
   launcher — you cannot move a cursor to an entry and press Enter to open it.

2. **No meaningful structure.** All views are listed at the same level with no
   grouping. As the number of views grows this becomes hard to scan. "Home"
   itself appearing in the list is circular and confusing.

## Proposed change

Replace the flat table with a sectioned, keyboard-navigatable list:

- **Sections** group related views. Initial sections:
  - **Apps** — resource views (Queues, and future app-level views).
  - **System** — configuration and tooling views (Settings, Log).
- **Section headers** are non-selectable labels that visually separate groups.
- **Navigation** — arrow keys (or j/k) move the cursor between selectable
  entries; Enter activates the selected view (equivalent to typing its name in
  the command prompt).
- **"Home" is removed** from the list — it is the home screen itself, so
  listing it is circular.

Assumed section layout (to be confirmed before plan):

| Section | Entries |
|---------|---------|
| Apps    | Queues  |
| System  | Settings, Log |

## Scope

**In scope:**
- New home view implementation: sectioned layout, keyboard navigation, Enter
  to activate.
- Remove "home" from the entry list passed to `NewHome`.
- Update `reapplyTheme` / `RepaintHomeTable` as needed.
- Update tests.

**Out of scope:**
- Dynamically registering views into sections at runtime (sections are
  hard-coded for now).
- Any other view changes.
- Mouse support.
