# Tasks: Format JSON and XML snippet bodies

1. [x] Add the JSON/XML body formatter in `internal/snippet`, with focused tests for detection, indentation, unchanged invalid/plain text, and preservation of payload meaning and XML structure.
2. [ ] Normalize bodies at the shared `Store.Save` boundary and cover all snippet write behavior, including front-matter preservation and overwrite saves.
3. [ ] Normalize and persist a snippet when opening it in the library editor; keep preview and picker reads read-only, and cover write failures without opening the editor.
4. [ ] Update the message-snippet living spec and README, then run formatting, TUI tests, and manual checks for compact JSON/XML, invalid payloads, and plain text.
