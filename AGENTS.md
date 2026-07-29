# AGENTS.md

Always keep this file up to date with your learnings.

Always keep REQUIREMENTS.md up to date with RFC 2219 requirements for the project.

Always keep README.md up to date with the project overview and usage.

## Toolchain

- **Language**: Go 1.24+
- **TUI framework**: `github.com/charmbracelet/bubbletea`
- **TUI components**: `github.com/charmbracelet/bubbles`
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
- All TUI rendering goes through `tui.go` (bubbletea model).
- `main.go` is minimal: parse CLI arg, start program.
- Destroy confirmation is a modal overlay in the TUI.
- Error messages are displayed inline, not via debug logs.
- Commit messages are written in the imperative mood.