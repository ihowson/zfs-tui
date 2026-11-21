package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "zfs-tui: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("zfs-tui", flag.ContinueOnError)
	colorMode := flags.String("color", colorAuto, "color output: auto, always or never")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: zfs-tui [-color auto|always|never] [filesystem]\n\n")
		fmt.Fprintf(flags.Output(), "Browse ZFS datasets and destroy their snapshots.\n")
		fmt.Fprintf(flags.Output(), "Without an argument the TUI starts at the list of pools.\n")
		fmt.Fprintf(flags.Output(), "Requires root and the zfs CLI in PATH.\n")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	switch *colorMode {
	case colorAuto, colorAlways, colorNever:
	default:
		return fmt.Errorf("invalid -color %q (want auto, always or never)", *colorMode)
	}

	if flags.NArg() > 1 {
		flags.Usage()
		return errors.New("expected at most one ZFS filesystem argument")
	}
	fsName := flags.Arg(0)

	if err := checkPrivileges(); err != nil {
		return err
	}
	if fsName != "" {
		if err := checkFilesystem(fsName); err != nil {
			return err
		}
	}

	configureColor(*colorMode)

	_, err := tea.NewProgram(initialModel(fsName), tea.WithAltScreen()).Run()
	return err
}

// checkPrivileges fails unless the process can run ZFS commands.
func checkPrivileges() error {
	if os.Geteuid() != 0 {
		return errors.New("must run as root (try: sudo zfs-tui [filesystem])")
	}
	if _, err := exec.LookPath("zfs"); err != nil {
		return fmt.Errorf("zfs CLI not found in PATH: %w", err)
	}
	return nil
}
