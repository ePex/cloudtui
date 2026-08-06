# Tasks — Bugfix 18: Settings theme dropdown not interactive

Spec: [spec.md](spec.md)

1. [x] **Focus fix** — `switchTo` changed to `a.tv.SetFocus(v.Primitive())`
   instead of `a.tv.SetFocus(a.pages)`; verified all existing view tests pass.

2. [x] **Manual verification** — opened settings with `s`, pressed Enter on
   the Theme dropdown, navigated to "cyberpunk", pressed Enter — theme applied.
