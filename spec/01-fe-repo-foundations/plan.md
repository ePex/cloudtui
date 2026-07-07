# Plan — Repo foundations

Spec: [spec.md](spec.md)

## Approach

Establish the top-level directory layout and the Go module structure inside `tui/` following standard Go conventions (`cmd/` for the entry point, `internal/` for application packages).

## Files touched

- `tui/cmd/tui/main.go` (new — entry point)
- `tui/go.mod`, `tui/go.sum` (new)

## Testing

Directory layout and module structure are structural, not behavioral — no unit tests apply here. The layout is verified implicitly by `go build` succeeding.
