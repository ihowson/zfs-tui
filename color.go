package main

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Accepted values for the -color flag.
const (
	colorAuto   = "auto"
	colorAlways = "always"
	colorNever  = "never"
)

// configureColor sets the process-wide lipgloss color profile.
func configureColor(mode string) {
	lipgloss.SetColorProfile(colorProfile(mode))
}

func colorProfile(mode string) termenv.Profile {
	switch mode {
	case colorNever:
		return termenv.Ascii
	case colorAlways:
		return termenv.TrueColor
	}
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}
	if force := os.Getenv("CLICOLOR_FORCE"); force != "" && force != "0" {
		return termenv.TrueColor
	}
	if p := lipgloss.ColorProfile(); p != termenv.Ascii {
		return p
	}
	// TERM is often missing when the TUI runs under sudo's env_reset, and
	// lipgloss then reports no color. A full-screen TUI only starts on a real
	// terminal, so assume 256 colors instead of rendering everything plain.
	if isTerminal(os.Stdout) && os.Getenv("TERM") != "dumb" {
		return termenv.ANSI256
	}
	return termenv.Ascii
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
