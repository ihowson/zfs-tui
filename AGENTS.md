# AGENTS.md

Always keep this file up to date with your learnings.

Always keep REQUIREMENTS.md up to date with project requirements in RFC 2219 format.

Always keep README.md up to date with the project overview and usage.

## Toolchain

- **Language**: Go 1.24+
- **TUI framework**: `github.com/charmbracelet/bubbletea`
- **Styling**: `github.com/charmbracelet/lipgloss`
- **ZFS interface**: `github.com/mistifyio/go-zfs/v4` (shells out to `zfs` CLI, no CGO)

## Build & Verify

```sh
export GOTOOLCHAIN=go1.24.2
go build -o zfs-tui .
go vet ./...
go mod tidy
```

## Test Setup

- Test pool: `sudo /tmp/setup-zfs-test.sh` (file-backed, no real disks needed)
- Usage: `sudo ./zfs-tui testpool/data`
- Teardown: `sudo /tmp/setup-zfs-test.sh --teardown testpool`

## Conventions

- All ZFS interaction goes through `snapshot.go` (wraps `go-zfs`).
- `snapshot.go` also provides `poolAvail` and `poolName` helpers.
- All TUI rendering goes through `tui.go` (bubbletea model).
- `main.go` is minimal: parse CLI arg, check EUID, start program.
- Destroy uses async message chaining (`destroyNextMsg` → goroutine → `destroyResultMsg`) for inline progress display.
- Messages sent to the event loop are the sole mechanism for model mutations — no goroutine directly touches the model.
- Cursor is clamped to `[0, len(snapshots)-1]` after any list reload.
- Error messages are displayed inline, not via debug logs.
- Commit messages are written in the imperative mood.