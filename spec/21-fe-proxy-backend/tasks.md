# Tasks — FE 21: mq-proxy backend for the TUI

1. [x] ~~Add `Backend` field and `ProxyConfig` struct to `tui/internal/config/config.go`; update `Default()`.~~
   Superseded — `ProxyConfig` was added, but hangs off `config.Connection`
   (FE 22) instead of a top-level `Config.Backend` field. See spec 22.
2. [x] ~~Add config tests for new fields to `tui/internal/config/config_test.go`.~~
   Superseded — covered by FE 22's config tests instead.
3. [x] Create `tui/internal/queue/proxy/proxy.go` implementing all 7 `queue.Backend` methods.
4. [x] Create `tui/internal/queue/proxy/proxy_test.go` with `httptest`-based unit tests.
5. [x] ~~Update `tui/internal/app/app.go` to select backend based on `cfg.Backend`.~~
   Superseded — backend selection lives in `newBackendForConn(conn
   config.Connection)`, added as part of FE 22.
6. [x] ~~Update `tui/config.example.yaml` to document `backend` and `proxy` config sections.~~
   Superseded — documented as part of FE 22's `connections` shape instead.
7. [x] Run `go build ./...` and `go test ./...` in `tui/` to verify all tasks pass.
