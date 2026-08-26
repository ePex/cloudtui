# cloudtui

A terminal UI for managing cloud resources.

## Installing a release

**macOS/Linux:**

```
curl -fsSL https://raw.githubusercontent.com/ePex/cloudtui/main/scripts/install.sh | sh
```

**Windows (PowerShell):**

```
irm https://raw.githubusercontent.com/ePex/cloudtui/main/scripts/install.ps1 | iex
```

Both scripts install to a per-user directory (no `sudo`/admin rights
needed) and verify the download's checksum before installing. Pin a
specific version with the `CLOUDTUI_VERSION` environment variable
(default: latest).

Prefer to pick the archive yourself? Download it for your OS/architecture from the
[Releases](https://github.com/ePex/cloudtui/releases) page
(`cloudtui_<version>_<os>_<arch>.tar.gz`, or `.zip` on Windows), extract
it, and run the `cloudtui` binary inside. `checksums.txt` on the same
release lets you verify the download. Prebuilt for linux/darwin/windows
on amd64/arm64.

Prefer to build from source instead? See "Usage" below.

## Usage

```
task run:tui
```

Press `:q` to quit.

## Status

Early development. See `CLAUDE.md` for repository conventions.

## License

[MIT](LICENSE)
