package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type pane int

const (
	paneDatasets pane = iota
	paneSnapshots
)

const (
	minWidth  = 50
	minHeight = 10
)

var (
	// accent marks titles and the cursor.
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	// pathStyle is used for dataset names in the header and pool context row.
	pathStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	// poolStyle colours the pool context row.
	poolStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	busyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dangerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

	borderFocus = lipgloss.Color("212")
	borderBlur  = lipgloss.Color("238")
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Switch  key.Binding
	Descend key.Binding
	Ascend  key.Binding
	Select  key.Binding
	Destroy key.Binding
	Refresh key.Binding
	Quit    key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Switch:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
		Descend: key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("enter", "descend")),
		Ascend:  key.NewBinding(key.WithKeys("backspace", "h"), key.WithHelp("backspace", "up")),
		Select:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "select")),
		Destroy: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "destroy")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

type model struct {
	root    string // dataset given on the command line; browse boundary
	current string // dataset being browsed

	children   []datasetItem
	datasetCur int
	snapshots  []snapshotItem
	snapCur    int
	selected   map[string]bool // keyed by full snapshot name

	pane   pane
	width  int
	height int

	loadingDatasets  bool
	loadingSnapshots bool
	err              error
	status           string
	statusOK         bool
	avail            uint64

	confirming  bool
	destroyQ    []string
	destroySize int
	destroyDone int
	destroying  string

	keys keyMap
	spin spinner.Model
}

func initialModel(root string) model {
	spin := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	spin.Style = busyStyle
	return model{
		root:            root,
		current:         root,
		selected:        make(map[string]bool),
		loadingDatasets: true,
		keys:            defaultKeyMap(),
		spin:            spin,
	}
}

// datasetsMsg carries the child datasets of one dataset. Results for a dataset
// that is no longer current are discarded.
type datasetsMsg struct {
	dataset  string
	children []datasetItem
	avail    uint64
	err      error
}

// snapshotsMsg carries the snapshots of one dataset, loaded after its child
// datasets so the browser becomes usable as early as possible.
type snapshotsMsg struct {
	dataset   string
	snapshots []snapshotItem
	err       error
}

type destroyResultMsg struct {
	name string
	err  error
}

func (m model) Init() tea.Cmd {
	_, cmd := m.startLoad()
	return cmd
}

// startLoad begins loading the current dataset's children; its snapshots are
// queued as soon as the children arrive.
func (m model) startLoad() (model, tea.Cmd) {
	m.loadingDatasets = true
	m.loadingSnapshots = false
	return m, tea.Batch(loadDatasetsCmd(m.current), m.spin.Tick)
}

// busy reports whether a ZFS operation is in flight.
func (m model) busy() bool {
	return m.loadingDatasets || m.loadingSnapshots || m.destroying != ""
}

// datasetLabel names what the browser is loading.
func (m model) datasetLabel() string {
	if m.current == "" {
		return "pools"
	}
	return m.current
}

func (m model) spinnerLine(text string, w int) string {
	return pad(m.spin.View()+" "+dimStyle.Render(text), w)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if !m.busy() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case datasetsMsg:
		if msg.dataset != m.current {
			return m, nil
		}
		m.loadingDatasets = false
		if msg.err != nil {
			m.err = msg.err
			m.loadingSnapshots = false
			return m, nil
		}
		m.err = nil
		m.children = msg.children
		m.avail = msg.avail
		m.clampCursors()
		if msg.dataset == "" {
			m.snapshots = nil
			return m, nil
		}
		m.loadingSnapshots = true
		return m, loadSnapshotsCmd(msg.dataset)

	case snapshotsMsg:
		if msg.dataset != m.current {
			return m, nil
		}
		m.loadingSnapshots = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.snapshots = msg.snapshots
		m.pruneSelection()
		m.clampCursors()

	case destroyResultMsg:
		m.destroying = ""
		if msg.err != nil {
			m.status = fmt.Sprintf("destroy failed: %v", msg.err)
			m.statusOK = false
			m.destroyQ = nil
			m.destroySize, m.destroyDone = 0, 0
			return m.startLoad()
		}
		delete(m.selected, msg.name)
		m.destroyDone++
		if len(m.destroyQ) > 1 {
			m.destroyQ = m.destroyQ[1:]
			m.destroying = m.destroyQ[0]
			return m, tea.Batch(destroyCmd(m.destroyQ[0]), m.spin.Tick)
		}
		m.destroyQ = nil
		m.status = fmt.Sprintf("destroyed %d snapshot(s)", m.destroyDone)
		m.statusOK = true
		m.destroySize, m.destroyDone = 0, 0
		return m.startLoad()
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if len(m.destroyQ) > 0 {
		return m, nil
	}

	if m.confirming {
		switch msg.String() {
		case "y", "Y":
			m.confirming = false
			m.destroyQ = m.selectedNames()
			m.destroySize, m.destroyDone = len(m.destroyQ), 0
			if len(m.destroyQ) == 0 {
				return m, nil
			}
			m.destroying = m.destroyQ[0]
			return m, tea.Batch(destroyCmd(m.destroyQ[0]), m.spin.Tick)
		case "n", "N", "esc":
			m.confirming = false
		}
		return m, nil
	}

	// Clearing the status must not swallow the keypress.
	m.status = ""

	switch {
	case key.Matches(msg, m.keys.Up):
		m.move(-1)
	case key.Matches(msg, m.keys.Down):
		m.move(1)
	case key.Matches(msg, m.keys.Switch):
		m.pane = paneSnapshots - m.pane
	case key.Matches(msg, m.keys.Descend):
		if m.pane == paneDatasets {
			return m.descend()
		}
	case key.Matches(msg, m.keys.Ascend):
		if m.pane == paneDatasets {
			return m.ascend()
		}
	case key.Matches(msg, m.keys.Select):
		if m.pane == paneSnapshots && len(m.snapshots) > 0 {
			name := m.snapshots[m.snapCur].Name
			if m.selected[name] {
				delete(m.selected, name)
			} else {
				m.selected[name] = true
			}
		}
	case key.Matches(msg, m.keys.Destroy):
		if m.pane == paneSnapshots && len(m.selected) > 0 {
			m.confirming = true
		}
	case key.Matches(msg, m.keys.Refresh):
		return m.startLoad()
	}

	return m, nil
}

func (m *model) move(delta int) {
	if m.pane == paneDatasets {
		m.datasetCur = clamp(m.datasetCur+delta, 0, max(0, len(m.rows())-1))
		return
	}
	m.snapCur = clamp(m.snapCur+delta, 0, max(0, len(m.snapshots)-1))
}

func (m model) descend() (tea.Model, tea.Cmd) {
	rows := m.rows()
	if len(rows) == 0 {
		return m, nil
	}
	return m.navigate(rows[clamp(m.datasetCur, 0, len(rows)-1)].Name)
}

func (m model) ascend() (tea.Model, tea.Cmd) {
	parent, ok := parentDataset(m.current, m.root)
	if !ok {
		return m, nil
	}
	return m.navigate(parent)
}

func (m model) navigate(target string) (tea.Model, tea.Cmd) {
	m.current = target
	m.children = nil
	m.snapshots = nil
	m.datasetCur, m.snapCur = 0, 0
	m.selected = make(map[string]bool)
	m.err = nil
	m.status = ""
	return m.startLoad()
}

func loadDatasetsCmd(dataset string) tea.Cmd {
	return func() tea.Msg {
		return loadDatasets(dataset)
	}
}

func loadDatasets(dataset string) datasetsMsg {
	if dataset == "" {
		pools, err := listPools()
		return datasetsMsg{dataset: "", children: pools, err: err}
	}
	children, err := listChildren(dataset)
	if err != nil {
		return datasetsMsg{dataset: dataset, err: err}
	}
	avail, _ := poolAvail(dataset)
	return datasetsMsg{dataset: dataset, children: children, avail: avail}
}

func loadSnapshotsCmd(dataset string) tea.Cmd {
	return func() tea.Msg {
		snapshots, err := listSnapshots(dataset)
		return snapshotsMsg{dataset: dataset, snapshots: snapshots, err: err}
	}
}

func destroyCmd(name string) tea.Cmd {
	return func() tea.Msg {
		return destroyResultMsg{name: name, err: destroySnapshot(name)}
	}
}

// rows returns the dataset pane entries: an ".." entry when the browser is
// below the root dataset, followed by the immediate children of current.
func (m model) rows() []datasetItem {
	rows := make([]datasetItem, 0, len(m.children)+1)
	if parent, ok := parentDataset(m.current, m.root); ok {
		rows = append(rows, datasetItem{Name: parent, up: true})
	}
	return append(rows, m.children...)
}

func parentDataset(name, root string) (string, bool) {
	if name == root {
		return "", false
	}
	if i := strings.LastIndexByte(name, '/'); i > 0 {
		return name[:i], true
	}
	// A pool is the child of the virtual root that lists all pools.
	if root == "" {
		return "", true
	}
	return "", false
}

func (m *model) pruneSelection() {
	for name := range m.selected {
		found := false
		for _, s := range m.snapshots {
			if s.Name == name {
				found = true
				break
			}
		}
		if !found {
			delete(m.selected, name)
		}
	}
}

func (m *model) clampCursors() {
	m.datasetCur = clamp(m.datasetCur, 0, max(0, len(m.rows())-1))
	m.snapCur = clamp(m.snapCur, 0, max(0, len(m.snapshots)-1))
}

func (m model) selectedNames() []string {
	names := make([]string, 0, len(m.selected))
	for _, s := range m.snapshots {
		if m.selected[s.Name] {
			names = append(names, s.Name)
		}
	}
	return names
}

func (m model) View() string {
	if m.width == 0 && m.height == 0 {
		return dimStyle.Render("loading…")
	}
	if m.width < minWidth || m.height < minHeight {
		return dimStyle.Render(fmt.Sprintf(
			"terminal too small: %dx%d (minimum %dx%d)", m.width, m.height, minWidth, minHeight))
	}

	header := accentStyle.Render("zfs-tui") + " "
	if m.current == "" {
		header += pathStyle.Render("pools") + dimStyle.Render("  select a pool to browse")
	} else {
		header += pathStyle.Render(m.current) + dimStyle.Render("  Free: "+formatBytes(m.avail))
	}
	switch {
	case m.loadingDatasets:
		header += "  " + m.spin.View() + busyStyle.Render(" loading "+m.datasetLabel()+"…")
	case m.loadingSnapshots:
		header += "  " + m.spin.View() + busyStyle.Render(" loading snapshots…")
	}

	innerH := max(1, m.height-4)
	leftW := min(30, max(16, m.width/4))
	rightW := max(16, m.width-leftW-5)

	left := renderPane(m.datasetTitle(), m.datasetLines(leftW, innerH), leftW, innerH, m.pane == paneDatasets)
	right := renderPane(m.snapshotTitle(), m.snapshotLines(rightW, innerH), rightW, innerH, m.pane == paneSnapshots)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	footer := m.footer()
	return strings.Join([]string{pad(header, m.width), body, pad(footer, m.width)}, "\n")
}

func (m model) datasetTitle() string {
	if m.current == "" {
		return fmt.Sprintf("Pools (%d)", len(m.children))
	}
	return fmt.Sprintf("Datasets (%d)", len(m.children))
}

func (m model) snapshotTitle() string {
	if m.current == "" {
		return "Snapshots"
	}
	n := len(m.snapshots)
	if sel := len(m.selected); sel > 0 {
		return fmt.Sprintf("Snapshots (%d, %d selected) · used / refer", n, sel)
	}
	return fmt.Sprintf("Snapshots (%d) · used / refer", n)
}

// datasetLines renders the visible dataset rows for a pane of the given size.
func (m model) datasetLines(w, h int) []string {
	lines := make([]string, 0, h)
	if h <= 2 {
		return nil
	}
	area := h - 2
	// Pool context row: where in the tree the browser currently is.
	if m.current != "" && area > 0 {
		lines = append(lines, pad(dimStyle.Render("pool ")+poolStyle.Render(poolName(m.current)), w))
		area--
	}

	rows := m.rows()
	switch {
	case m.loadingDatasets && len(rows) == 0:
		lines = append(lines, m.spinnerLine("loading "+m.datasetLabel()+"…", w))
	case m.current == "" && len(rows) == 0:
		lines = append(lines, dimStyle.Render("no pools"))
	case len(rows) == 0:
		lines = append(lines, dimStyle.Render("no child datasets"))
	default:
		start, end := window(len(rows), m.datasetCur, area)
		for i := start; i < end; i++ {
			row := rows[i]
			label := row.Name
			switch {
			case row.up:
				label = "↑ .."
			default:
				if j := strings.LastIndexByte(label, '/'); j >= 0 {
					label = label[j+1:]
				}
			}
			marker := "  "
			switch {
			case i == m.datasetCur:
				marker = accentStyle.Render("▸ ")
				label = accentStyle.Render(label)
			case row.up:
				label = dimStyle.Render(label)
			default:
				label = dirStyle.Render(label)
			}
			lines = append(lines, pad(marker+label, w))
		}
	}
	for len(lines) < h-2 {
		lines = append(lines, "")
	}
	return lines
}

// snapshotLines renders the visible snapshot rows for a pane of the given size.
func (m model) snapshotLines(w, h int) []string {
	lines := make([]string, 0, h)
	if h <= 2 {
		return nil
	}
	area := h - 2
	switch {
	case m.current == "":
		lines = append(lines, dimStyle.Render("select a pool"))
	case m.loadingSnapshots && len(m.snapshots) == 0:
		lines = append(lines, m.spinnerLine("loading snapshots…", w))
	case len(m.snapshots) == 0:
		lines = append(lines, dimStyle.Render("no snapshots"))
	default:
		start, end := window(len(m.snapshots), m.snapCur, area)
		for i := start; i < end; i++ {
			lines = append(lines, snapshotLine(m.snapshots[i], i == m.snapCur, m.selected[m.snapshots[i].Name], w))
		}
	}
	for len(lines) < h-2 {
		lines = append(lines, "")
	}
	return lines
}

func snapshotLine(s snapshotItem, cursor, selected bool, w int) string {
	marker := "  "
	if cursor {
		marker = accentStyle.Render("▸ ")
	}
	box := dimStyle.Render("[ ] ")
	if selected {
		box = selectedStyle.Render("[x] ")
	}
	sizes := fmt.Sprintf("%10s  %10s", formatBytes(s.Used), formatBytes(s.Refer))
	nameW := w - 2 - 4 - len(sizes) - 2
	if nameW >= 8 {
		name := s.Short
		if selected {
			name = selectedStyle.Render(name)
		} else if cursor {
			name = pathStyle.Render(name)
		}
		return pad(marker+box+pad(name, nameW)+"  "+dimStyle.Render(sizes), w)
	}
	name := s.Short
	if selected {
		name = selectedStyle.Render(name)
	} else if cursor {
		name = pathStyle.Render(name)
	}
	return pad(marker+box+pad(name, w-2-4), w)
}

func (m model) footer() string {
	var line string
	switch {
	case m.destroying != "":
		line = m.spin.View() + " " + busyStyle.Render(fmt.Sprintf("destroying %s… (%d/%d)",
			shortSnapshotName(m.destroying), m.destroyDone+1, m.destroySize))
	case m.confirming:
		line = dangerStyle.Render(fmt.Sprintf("Destroy %d selected snapshot(s)? (y/n)", len(m.selected)))
	case m.err != nil:
		line = dangerStyle.Render("Error: " + m.err.Error())
	case m.status != "":
		if m.statusOK {
			line = okStyle.Render(m.status)
		} else {
			line = dangerStyle.Render(m.status)
		}
	default:
		line = dimStyle.Render("tab pane · enter descend · bksp up · x select · D destroy · r refresh · q quit")
	}
	return pad(line, m.width)
}

func renderPane(title string, body []string, w, h int, focused bool) string {
	lines := make([]string, 0, h)
	titleStyle := dimStyle
	if focused {
		titleStyle = accentStyle
	}
	lines = append(lines, pad(titleStyle.Render(title), w))
	if h > 1 {
		lines = append(lines, pad(dimStyle.Render(strings.Repeat("─", w)), w))
	}
	lines = append(lines, body...)
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	lines = lines[:h]

	borderColor := borderBlur
	if focused {
		borderColor = borderFocus
	}
	style := lipgloss.NewStyle().
		Width(w).
		Height(h).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
	return style.Render(strings.Join(lines, "\n"))
}

// window returns the slice of a list that keeps cursor visible in h rows.
func window(total, cursor, h int) (start, end int) {
	if total <= h {
		return 0, total
	}
	start = clamp(cursor-h/2, 0, total-h)
	return start, start + h
}

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// pad truncates s to w visible cells and right-pads it with spaces.
func pad(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	if n := w - ansi.StringWidth(s); n > 0 {
		s += strings.Repeat(" ", n)
	}
	return s
}

func shortSnapshotName(name string) string {
	if _, short, ok := strings.Cut(name, "@"); ok {
		return short
	}
	return name
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
