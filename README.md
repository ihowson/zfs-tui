# zfs-tui

A terminal UI for browsing ZFS datasets and destroying snapshots.

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [go-zfs](https://github.com/mistifyio/go-zfs) (no CGO required).

## Requirements

- Linux with the `zfs` CLI in `PATH`
- root privileges, since `zfs destroy` needs them

The program exits non-zero before starting the UI when it is not root, when the `zfs` CLI is missing, or when the argument is not an existing ZFS filesystem.

## Usage

```sh
sudo ./zfs-tui [-color auto|always|never] [filesystem]
```

`-color` defaults to `auto`: it honours `NO_COLOR` and `CLICOLOR_FORCE`, uses the terminal's reported capability, and falls back to 256 colors on a TTY when that detection reports no color (e.g. a stripped `TERM`). Use `-color always` to force color when `NO_COLOR` is set, `-color never` to disable it.

Without an argument the TUI starts at the list of pools:

```sh
sudo ./zfs-tui
```

With an argument it starts at that filesystem:

```sh
sudo ./zfs-tui tank/data
```

## Browsing

```
zfs-tui  tank/data  Free: 233.0 MiB
╭─────────────────────────╮ ╭──────────────────────────────────────────────────────────────────────╮
│Datasets (3)             │ │Snapshots (4, 1 selected) · used / refer                              │
│─────────────────────────│ │──────────────────────────────────────────────────────────────────────│
│pool tank                │ │  [ ] media-2026-11-20                             1.0 GiB   256.0 KiB│
│▸ backups                │ │  [ ] media-2026-11-19                           512.0 MiB   128.0 KiB│
│  media                  │ │▸ [x] weekly-2026-11-17                            3.0 GiB    64.0 MiB│
│  vm                     │ │  [ ] weekly-2026-11-10                                0 B         0 B│
╰─────────────────────────╯ ╰──────────────────────────────────────────────────────────────────────╯
tab pane · enter descend · bksp up · x select · D destroy · r refresh · q quit
```

The dataset pane lists the immediate child datasets of the dataset named in the header; `..` appears while you are below the starting point. Started without an argument, the header reads `pools` and the pane lists every pool; descending into a pool shows its filesystems, and `..` returns to the pool list. With an argument the starting point is that filesystem, and `..` never ascends above it.

The snapshot pane always shows the snapshots of the dataset named in the header, and asks you to select a pool while none is selected. Selection is kept by snapshot name, so it survives refreshes.

Child datasets are loaded and drawn before the snapshots of the current dataset are queried, so the browser is usable while the snapshot listing is still in flight. Child listings are cached per dataset: revisiting a dataset redraws its children immediately and only reloads the snapshots. Press `r` to discard the cache and refetch the current dataset.

The dataset pane starts with a `pool` row naming the pool you are inside, so the location in the tree stays visible while you descend.

## Colors and progress

Rendering uses a fixed palette: magenta accent for the cursor and pane titles, blue for the pool row, cyan for child datasets, green for selection and success messages, amber for work in flight, red for errors and destroy confirmation, and dim gray for secondary text. The focused pane has a magenta border and title; the unfocused one is gray.

Every `zfs` call runs asynchronously, and anything in flight is animated so the UI never appears stalled:

```
zfs-tui tank/data  Free: 233.0 MiB  ⠋ loading tank/data…
...
│pool tank                │ │⠋ loading snapshots…                                           │
```

Destroys show `⠋ destroying snap-b… (2/4)` in the footer while the queue drains.

## Key Bindings

| Key | Action |
|-----|--------|
| `tab` | Switch focus between dataset and snapshot panes |
| `j` / `↓` | Move cursor down in the focused pane |
| `k` / `↑` | Move cursor up in the focused pane |
| `enter` / `l` | Descend into the selected dataset (dataset pane) |
| `backspace` / `h` | Ascend to the parent dataset (dataset pane) |
| `x` | Toggle selection on current snapshot (snapshot pane) |
| `D` | Destroy all selected snapshots, after confirmation (snapshot pane) |
| `r` | Discard the dataset cache and refresh the current dataset |
| `q` / `ctrl+c` | Quit |

Destroy prompts with `Destroy N selected snapshot(s)? (y/n):`; press `y` to confirm or `n`/`esc` to cancel. Each destroy is displayed inline: `Destroying snap-b… (2/4)`.

Terminals smaller than 50x10 show a message instead of the panes.

## Build

```sh
export GOTOOLCHAIN=go1.24.2
go build -o zfs-tui .
```

Requires Go 1.24+.

To cross-compile static Linux binaries:

```sh
./build.sh
```

Outputs `dist/zfs-tui-linux-amd64` and `dist/zfs-tui-linux-arm64` (`OUT=... ./build.sh` to override the directory). Builds are `CGO_ENABLED=0`, stripped, and path-trimmed.

## Testing

Unit tests cover the browser state machine and layout; they do not need a ZFS pool:

```sh
go test ./...
```

For manual testing on a real pool, use a file-backed ZFS test pool (see `AGENTS.md` for a helper script pattern), e.g.:

```sh
sudo /tmp/setup-zfs-test.sh               # creates testpool/data with 5 snapshots
sudo ./zfs-tui testpool/data              # launch TUI
sudo /tmp/setup-zfs-test.sh --teardown    # clean up
```
