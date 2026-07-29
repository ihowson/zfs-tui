# zfs-tui

A terminal UI for listing, selecting, and destroying ZFS snapshots.

Built with [bubbletea](https://github.com/charmbracelet/bubbletea) and [go-zfs](https://github.com/mistifyio/go-zfs) (no CGO required).

## Usage

```sh
sudo ./zfs-tui <filesystem>
```

Example:

```sh
sudo ./zfs-tui tank/data
```

If run without root privileges, a warning is printed on stderr.

## Key Bindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `x` | Toggle selection on current snapshot |
| `D` | Destroy all selected snapshots (with confirmation) |
| `r` | Refresh snapshot list and pool free space |
| `q` | Quit |

Destroy prompts with `Destroy selected snapshots? (y/n):` — press `y` to confirm or `n` to cancel. Each destroy is displayed inline: `Destroying snap-b... (2/4)`.

## Display

```
Snapshots of tank/data  Free: 233.5 MiB

> [ ] snap-a                          Used: 1.2 MiB    Refer: 256 KiB
  [x] snap-b                          Used: 512 KiB    Refer: 128 KiB
  [ ] snap-c                          Used: 0 B        Refer: 0 B

j/k navigate · x select · D destroy · r refresh · q quit
```

## Build

```sh
export GOTOOLCHAIN=go1.24.2
go build -o zfs-tui .
```

Requires Go 1.24+.

## Testing

A helper script creates a file-backed ZFS test pool:

```sh
sudo /tmp/setup-zfs-test.sh               # creates testpool/data with 5 snapshots
sudo ./zfs-tui testpool/data              # launch TUI
sudo /tmp/setup-zfs-test.sh --teardown    # clean up
```