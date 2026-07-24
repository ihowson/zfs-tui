package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	confirmStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)

type model struct {
	fsName     string
	snapshots  []snapshotItem
	selected   map[int]bool
	cursor     int
	loading    bool
	err        error
	confirming bool
	msg        string
}

func initialModel(fsName string) model {
	return model{
		fsName:   fsName,
		selected: make(map[int]bool),
		loading:  true,
	}
}

type loadedMsg struct {
	snapshots []snapshotItem
	err       error
}

type destroyDoneMsg struct {
	err error
	n   int
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		snaps, err := listSnapshots(m.fsName)
		return loadedMsg{snapshots: snaps, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.msg != "" {
			m.msg = ""
			return m, nil
		}
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirming = false
				return m, m.destroySelected()
			case "n", "N", "esc":
				m.confirming = false
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.snapshots)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "x":
			if len(m.snapshots) > 0 {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "D":
			if len(m.snapshots) > 0 {
				for _, v := range m.selected {
					if v {
						m.confirming = true
						break
					}
				}
			}
		}

	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.snapshots = msg.snapshots
		}

	case destroyDoneMsg:
		if msg.err != nil {
			m.msg = fmt.Sprintf("destroy failed: %v", msg.err)
		} else {
			m.msg = fmt.Sprintf("destroyed %d snapshot(s)", msg.n)
			return m, func() tea.Msg {
				snaps, err := listSnapshots(m.fsName)
				return loadedMsg{snapshots: snaps, err: err}
			}
		}
	}

	return m, nil
}

func (m model) destroySelected() tea.Cmd {
	var toDestroy []int
	for i, v := range m.selected {
		if v {
			toDestroy = append(toDestroy, i)
		}
	}
	return func() tea.Msg {
		count := 0
		for _, idx := range toDestroy {
			if err := destroySnapshot(m.snapshots[idx].Name); err != nil {
				return destroyDoneMsg{err: err, n: count}
			}
			delete(m.selected, idx)
			count++
		}
		return destroyDoneMsg{err: nil, n: count}
	}
}

func (m model) View() string {
	if m.loading {
		return "Loading snapshots..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Snapshots of %s", m.fsName)))
	b.WriteString("\n\n")

	if len(m.snapshots) == 0 {
		b.WriteString(dimStyle.Render("no snapshots"))
		b.WriteString("\n\n")
	} else {
		for i, snap := range m.snapshots {
			prefix := "  "
			if i == m.cursor {
				prefix = cursorStyle.Render("> ")
			}
			sel := " "
			if m.selected[i] {
				sel = selectedStyle.Render("x")
			}
			used := formatBytes(snap.Used)
			snapName := snap.Name
			if idx := strings.Index(snapName, "@"); idx >= 0 {
				snapName = snapName[idx+1:]
			}
			line := fmt.Sprintf("%s[%s] %-40s %s", prefix, sel, snapName, used)
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if m.confirming {
		b.WriteString(confirmStyle.Render("Destroy selected snapshots? (y/n): "))
	} else if m.msg != "" {
		b.WriteString(dimStyle.Render(m.msg))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("j/k navigate · x select · D destroy · q quit"))
	}

	return b.String()
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GiB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KiB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}