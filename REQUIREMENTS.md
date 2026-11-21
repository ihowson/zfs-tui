# Requirements

Keywords follow RFC 2119: MUST, MUST NOT, SHOULD, MAY.

## 2026-11-20

- The CLI MUST accept zero or one argument; more than one MUST be a usage error
- With no argument the TUI MUST start at the list of pools; with an argument it MUST start at that filesystem
- The dataset pane at the pool level MUST list the root datasets (pools), and ascending from a pool MUST return to that list
- The snapshot pane MUST show a selection hint rather than snapshots while the pool level is active
- The program MUST exit non-zero with a clear message, before starting the TUI, when it lacks requisite permissions: the process is not root, the `zfs` CLI is not in `PATH`, or an argument was given that is not an existing ZFS filesystem
- The TUI MUST browse datasets interactively: the left pane lists the immediate child datasets of the current dataset, the right pane lists the snapshots of the current dataset
- The browser MUST allow descending into a child dataset and ascending to the parent dataset; ascending MUST stop at the starting point: the pool list when no argument was given, otherwise the filesystem from the command line
- The `..` row MUST be shown in the dataset pane only while the current dataset is below the root dataset, and MUST be the first row
- `tab` MUST switch focus between panes; navigation, selection, and destroy keys MUST apply to the focused pane only
- Snapshot selection MUST be keyed by full snapshot name, so a selection survives refresh, reordering, and child datasets being browsed
- Stale load results for a dataset that is no longer current MUST be discarded
- The view MUST reflow to the terminal size and MUST show a clear message when the terminal is smaller than 50x10
- The TUI MUST use bubbletea key bindings (`bubbles/key`) rather than raw string dispatch
- The dataset pane MUST show the `pool` context row above the child datasets whenever a dataset is being browsed, and MUST NOT show it at the pool level
- In-flight loads and destroys MUST be visibly animated (spinner plus `loading <target>…` or destroy progress) and MUST NOT render empty-state text while a load is pending
- The TUI MUST use a consistent palette: accent for cursor and titles, blue for the pool row, cyan for child datasets, green for selection and success, amber for busy states, red for errors and confirmation, dim for secondary text
- The focused pane MUST be visually distinct from the unfocused pane
- Child datasets MUST be loaded and rendered before the snapshots of the same dataset are requested, and the snapshot pane MUST show progress while that request is in flight
- Child dataset listings MUST be cached per dataset and reused when navigating back to a dataset, without refetching it; snapshots MUST still be reloaded
- The cache MUST hold child listings only; pool free space MUST be re-read whenever snapshots are loaded
- `r` MUST discard the cached dataset listings and refetch the current dataset
- The CLI MUST accept `-color auto|always|never`; `auto` MUST honour `NO_COLOR` and `CLICOLOR_FORCE` and MUST fall back to a 256-color profile on a TTY when environment-based detection reports no color support

## 2026-07-26

- Pool free space displayed in the title bar and refreshes on `r`
- `r` key refreshes the snapshot list and pool free space
- Cursor is clamped to valid range after any list reload
- Message dismissal does not consume the keypress that cleared it

## 2026-07-24

- Show both `used` and `refer` sizes in snapshot list
- Display progress during destroy operations (snapshot name + count)
- Warn user if not running as root
- Batch destroy correctly removes all selected snapshots in sequence

## 2026-07-22

- CLI takes a ZFS filesystem name as a positional argument
- TUI lists snapshots for the given filesystem, sorted by name
- Each row shows snapshot name and disk space consumed
- Mutt-style keybindings: `j`/`k` navigate, `x` toggle select, `D` destroy selected (with `y`/`n` confirmation), `r` refresh, `q` quit
- Destroy requires a confirmation dialog before executing the `zfs destroy` command
- Multi-select is supported for batch destroy
- All ZFS operations shell out to the `zfs` CLI via `go-zfs` (no CGO needed)
- Platform is Linux only
