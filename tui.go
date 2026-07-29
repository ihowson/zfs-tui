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
	fsName        string
	snapshots     []snapshotItem
	selected      map[int]bool
	cursor        int
	loading       bool
	err           error
	confirming    bool
	msg           string
	destroyQueue  []string
	destroyTotal  int
	destroyDone   int
	destroyingNow string
	poolAvail     uint64
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
	avail     uint64
	err       error
}

type destroyNextMsg struct{}
type destroyResultMsg struct {
	name string
	err  error
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		snaps, err := listSnapshots(m.fsName)
		var avail uint64
		if err == nil {
			avail, _ = poolAvail(m.fsName)
		}
		return loadedMsg{snapshots: snaps, avail: avail, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.msg != "" {
			m.msg = ""
		}
		if len(m.destroyQueue) > 0 {
			return m, nil
		}
		if m.confirming {
			switch msg.String() {
		case "y", "Y":
			m.confirming = false
			m.destroyQueue = nil
			m.destroyDone = 0
			m.destroyTotal = 0
			for i, v := range m.selected {
				if v {
					m.destroyQueue = append(m.destroyQueue, m.snapshots[i].Name)
				}
			}
			m.destroyTotal = len(m.destroyQueue)
			return m, func() tea.Msg {
				return destroyNextMsg{}
			}
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
		case "r":
			return m, func() tea.Msg {
				snaps, err := listSnapshots(m.fsName)
				var avail uint64
				if err == nil {
					avail, _ = poolAvail(m.fsName)
				}
				return loadedMsg{snapshots: snaps, avail: avail, err: err}
			}
		}

	case loadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.snapshots = msg.snapshots
			m.poolAvail = msg.avail
			if m.cursor >= len(m.snapshots) {
				m.cursor = len(m.snapshots) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
		}

	case destroyNextMsg:
		if len(m.destroyQueue) == 0 {
			m.msg = fmt.Sprintf("destroyed %d snapshot(s)", m.destroyDone)
			m.destroyDone = 0
			m.destroyTotal = 0
			return m, func() tea.Msg {
				snaps, err := listSnapshots(m.fsName)
				var avail uint64
				if err == nil {
					avail, _ = poolAvail(m.fsName)
				}
				return loadedMsg{snapshots: snaps, avail: avail, err: err}
			}
		}
		name := m.destroyQueue[0]
		m.destroyingNow = name
		return m, func() tea.Msg {
			err := destroySnapshot(name)
			return destroyResultMsg{name: name, err: err}
		}

	case destroyResultMsg:
		m.destroyingNow = ""
		if msg.err != nil {
			m.msg = fmt.Sprintf("destroy failed: %v", msg.err)
			m.destroyQueue = nil
			m.destroyDone = 0
			m.destroyTotal = 0
			return m, func() tea.Msg {
				snaps, err := listSnapshots(m.fsName)
				var avail uint64
				if err == nil {
					avail, _ = poolAvail(m.fsName)
				}
				return loadedMsg{snapshots: snaps, avail: avail, err: err}
			}
		}
		m.destroyDone++
		m.destroyQueue = m.destroyQueue[1:]
		for i, snap := range m.snapshots {
			if snap.Name == msg.name {
				delete(m.selected, i)
				break
			}
		}
		return m, func() tea.Msg {
			return destroyNextMsg{}
		}
	}

	return m, nil
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
	b.WriteString("  ")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Free: %s", formatBytes(m.poolAvail))))
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
			snapName := snap.Name
			if idx := strings.Index(snapName, "@"); idx >= 0 {
				snapName = snapName[idx+1:]
			}
			line := fmt.Sprintf("%s[%s] %-40s Used: %-10s Refer: %s",
				prefix, sel, snapName, formatBytes(snap.Used), formatBytes(snap.Refer))
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if m.destroyingNow != "" {
		done := m.destroyDone
		total := m.destroyTotal
		current := m.destroyingNow
		if idx := strings.Index(current, "@"); idx >= 0 {
			current = current[idx+1:]
		}
		b.WriteString(fmt.Sprintf("Destroying %s... (%d/%d)\n", current, done+1, total))
	} else if m.confirming {
		b.WriteString(confirmStyle.Render("Destroy selected snapshots? (y/n): "))
	} else if m.msg != "" {
		b.WriteString(dimStyle.Render(m.msg))
		b.WriteString("\n")
	} else {
		b.WriteString(dimStyle.Render("j/k navigate · x select · D destroy · r refresh · q quit"))
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