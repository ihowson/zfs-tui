# Requirements

## 2026-07-24

- Show both `used` and `refer` sizes in snapshot list
- Display progress during destroy operations (snapshot name + count)
- Warn user if not running as root
- Batch destroy correctly removes all selected snapshots in sequence

## 2026-07-22

- CLI takes a ZFS filesystem name as a positional argument
- TUI lists snapshots for the given filesystem, sorted by name
- Each row shows snapshot name and disk space consumed
- Mutt-style keybindings: `j`/`k` navigate, `x` toggle select, `D` destroy selected, `q` quit, `?` help
- Destroy requires a confirmation dialog before executing the `zfs destroy` command
- Multi-select is supported for batch destroy
- All ZFS operations shell out to the `zfs` CLI via `go-zfs` (no CGO needed)
- Platform is Linux only