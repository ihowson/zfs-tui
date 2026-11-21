# AGENTS.md

Always keep this file up to date with your learnings.

Always keep REQUIREMENTS.md up to date with project requirements in RFC 2119 format.

Always keep README.md up to date with the project overview and usage.

## Toolchain

- **Language**: Go 1.24+
- **TUI framework**: `github.com/charmbracelet/bubbletea`
- **Styling**: `github.com/charmbracelet/lipgloss`
- **Key bindings**: `github.com/charmbracelet/bubbles/key`
- **ZFS interface**: `github.com/mistifyio/go-zfs/v4` (shells out to `zfs` CLI, no CGO)

## Build & Verify

```sh
export GOTOOLCHAIN=go1.24.2
go build -o zfs-tui .
go vet ./...
go test ./...
./build.sh   # cross-compile static linux binaries (amd64, arm64) into dist/
go mod tidy
```

`go test ./...` needs no ZFS pool; it exercises the model, layout, and formatting.

## CI

`.github/workflows/ci.yml` and `.gitea/workflows/ci.yml` must stay in sync: gofmt check, `go vet`, `go test -race`, and `./build.sh` for both hosts. The Gitea copy omits the artifact upload (`actions/upload-artifact@v3` is rejected by actionlint and v4 needs a recent act_runner) and disables `setup-go` caching.

## Test Setup

- Test pool: `sudo /tmp/setup-zfs-test.sh` (file-backed, no real disks needed; recreate it if missing)
- Usage: `sudo ./zfs-tui testpool/data`
- Teardown: `sudo /tmp/setup-zfs-test.sh --teardown testpool`
- The program exits non-zero before starting the TUI without root, without `zfs` in `PATH`, or when the argument is not an existing ZFS filesystem.

## Conventions

- All ZFS interaction goes through `zfs.go` (wraps `go-zfs`): `checkFilesystem`, `listPools`, `listChildren`, `listSnapshots`, `poolAvail`, `poolName`, `destroySnapshot`.
- `main.go` parses flags and gates startup on `checkPrivileges`, plus `checkFilesystem` when an argument was given; the UI starts only when they pass. Zero arguments is valid and starts at the pool list.
- All TUI rendering and state live in `tui.go`; tests live in `tui_test.go`.
- The UI is a two-pane browser: dataset pane lists immediate children of `current`, snapshot pane lists snapshots of `current`. `root` is the browse boundary; `parentDataset` never ascends past it.
- `root` and `current` may both be `""`, the virtual pool level: `loadDataset("")` calls `listPools`, the pane shows root datasets, the snapshot pane shows a hint, and `parentDataset(pool, "")` returns `""` so `..` returns to the pool list. The argument is optional (`zfs-tui [filesystem]`).
- Keys are dispatched with `key.Matches` against `keyMap` bindings, not raw string switches.
- Palette lives in the `var` block at the top of `tui.go` (`accentStyle`, `pathStyle`, `poolStyle`, `dirStyle`, `dimStyle`, `selectedStyle`, `okStyle`, `busyStyle`, `dangerStyle`, `borderFocus`, `borderBlur`). Reuse these instead of new `lipgloss.Color` literals.
- Long-running work is animated with `bubbles/spinner`: every transition into a busy state returns `m.spin.Tick`, `spinner.TickMsg` is dropped when `busy()` is false. Loading panes show `spinnerLine(...)`, never the empty-state text.
- The snapshot pane title and the destroy prompt both include `current`, so the operator can never confuse which dataset's snapshots are listed or queued.
- Footer status uses `statusOK` to pick green/red; destroy failures must be red.
- `model.cache` maps a dataset name (including `""` for the pool list) to its `datasetCache`; it is filled by the `datasetsMsg` handler and read by `navigate`, which then only schedules the snapshot load. `r` clears the map. Snapshots and pool free space are never cached: both arrive in `snapshotsMsg`.
- Loading is staged: `startLoad` requests `datasetsMsg`; the `datasetsMsg` handler stores the children and returns `loadSnapshotsCmd`, and `snapshotsMsg` fills the snapshot pane. Do not fold the two back into one command — the browser must appear before snapshots resolve.
- `-color` (`color.go`) configures the lipgloss profile before the program starts: `auto` honours `NO_COLOR`/`CLICOLOR_FORCE` and falls back to `ANSI256` when stdout is a TTY but detection reports no color. Tests force the profile with `lipgloss.SetColorProfile` when they assert color output.
- Snapshot selection is a `map[string]bool` keyed by full snapshot name; prune it on every reload. Cursors are clamped after every reload.
- Destroy uses async message chaining (`destroyCmd` → goroutine → `destroyResultMsg`) for inline progress display; input is ignored while the queue is non-empty.
- Messages sent to the event loop are the sole mechanism for model mutations — no goroutine directly touches the model.
- Errors are shown inline in the footer, not via debug logs.
- Keep the view within terminal bounds: every rendered line must be exactly `width` cells, and the view exactly `height` lines (`TestViewFitsTerminal` enforces this).
- Commit messages are written in the imperative mood.
