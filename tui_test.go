package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func newTestModel() model {
	m := initialModel("tank/data")
	m.width, m.height = 80, 24
	return m
}

func press(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	return got
}

func runeKey(s string) tea.KeyMsg {
	if len([]rune(s)) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	}
	panic("unsupported key " + s)
}

func datasetItems(names []string) []datasetItem {
	items := make([]datasetItem, 0, len(names))
	for _, name := range names {
		items = append(items, datasetItem{Name: name})
	}
	return items
}

func snapshotItems(dataset string, names []string) []snapshotItem {
	items := make([]snapshotItem, 0, len(names))
	for _, name := range names {
		items = append(items, snapshotItem{Name: dataset + "@" + name, Short: name})
	}
	return items
}

// loadAll feeds the two load results the browser requests, in order.
func loadAll(t *testing.T, m model, dataset string, children, snapshots []string) model {
	t.Helper()
	m = press(t, m, datasetsMsg{dataset: dataset, children: datasetItems(children)})
	if dataset != "" {
		m = press(t, m, snapshotsMsg{dataset: dataset, snapshots: snapshotItems(dataset, snapshots)})
	}
	return m
}

func TestParentDataset(t *testing.T) {
	cases := []struct {
		name, root, want string
		ok               bool
	}{
		{"tank/data", "tank/data", "", false},
		{"tank/data/a", "tank/data", "tank/data", true},
		{"tank/data/a/b", "tank/data", "tank/data/a", true},
		{"tank", "", "", true},
		{"tank/data", "", "tank", true},
		{"", "", "", false},
	}
	for _, c := range cases {
		got, ok := parentDataset(c.name, c.root)
		if got != c.want || ok != c.ok {
			t.Errorf("parentDataset(%q, %q) = %q, %v; want %q, %v", c.name, c.root, got, ok, c.want, c.ok)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := map[uint64]string{
		0:           "0 B",
		1023:        "1023 B",
		1024:        "1.0 KiB",
		1536:        "1.5 KiB",
		1024 * 1024: "1.0 MiB",
		3 * 1 << 30: "3.0 GiB",
	}
	for in, want := range cases {
		if got := formatBytes(in); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestWindowKeepsCursorVisible(t *testing.T) {
	start, end := window(100, 50, 10)
	if end-start != 10 || !(start <= 50 && 50 < end) {
		t.Fatalf("window(100, 50, 10) = [%d,%d), cursor not visible in 10 rows", start, end)
	}
	if start, end := window(5, 4, 10); start != 0 || end != 5 {
		t.Fatalf("window(5, 4, 10) = [%d,%d), want [0,5)", start, end)
	}
	if start, _ := window(100, 99, 10); start != 90 {
		t.Fatalf("window bottom = %d, want 90", start)
	}
}

func TestBrowseDescendAndAscend(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", []string{"tank/data/a", "tank/data/b"}, []string{"s1"})

	// At the root there is no ".." row, so the cursor starts on the first child.
	m = press(t, m, runeKey("enter"))
	if m.current != "tank/data/a" {
		t.Fatalf("after enter, current = %q, want tank/data/a", m.current)
	}
	if len(m.children) != 0 || len(m.snapshots) != 0 {
		t.Fatalf("navigate must clear the old listing")
	}

	m = loadAll(t, m, "tank/data/a", nil, []string{"x"})
	if m.loadingDatasets || m.loadingSnapshots {
		t.Fatalf("load result must end loading state")
	}
	m = press(t, m, runeKey("backspace"))
	if m.current != "tank/data" {
		t.Fatalf("after backspace, current = %q, want tank/data", m.current)
	}

	// Root is the browse boundary.
	m = loadAll(t, m, "tank/data", nil, nil)
	m = press(t, m, runeKey("backspace"))
	if m.current != "tank/data" {
		t.Fatalf("ascend past root: current = %q, want tank/data", m.current)
	}
}

func TestEnterOnParentRowAscends(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data/a", nil, nil)
	m.current = "tank/data/a"

	m = press(t, m, runeKey("enter")) // cursor 0 is the ".." row
	if m.current != "tank/data" {
		t.Fatalf("enter on '..' gave current = %q, want tank/data", m.current)
	}
}

func TestSelectionSurvivesReloadByName(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", nil, []string{"s1", "s2"})
	m = press(t, m, runeKey("tab"))
	m = press(t, m, runeKey("j")) // second snapshot
	m = press(t, m, runeKey("x"))
	if len(m.selected) != 1 || !m.selected["tank/data@s2"] {
		t.Fatalf("selected = %v, want s2", m.selected)
	}

	// A refresh without s2 must drop the stale selection.
	m = loadAll(t, m, "tank/data", nil, []string{"s1"})
	if len(m.selected) != 0 {
		t.Fatalf("stale selection survived reload: %v", m.selected)
	}
}

func TestCursorClampedAfterReload(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", nil, []string{"s1", "s2", "s3", "s4", "s5"})
	m = press(t, m, runeKey("tab"))
	for range 4 {
		m = press(t, m, runeKey("j"))
	}
	if m.snapCur != 4 {
		t.Fatalf("snapCur = %d, want 4", m.snapCur)
	}
	m = loadAll(t, m, "tank/data", nil, []string{"s1", "s2"})
	if m.snapCur != 1 {
		t.Fatalf("snapCur after shrink = %d, want 1", m.snapCur)
	}
}

func TestStaleLoadIgnored(t *testing.T) {
	m := newTestModel()
	m.current = "tank/data/a"
	m = loadAll(t, m, "tank/data", nil, []string{"old"})
	if len(m.snapshots) != 0 {
		t.Fatalf("load result for a non-current dataset was applied")
	}
}

func TestStatusDoesNotSwallowKeypress(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", nil, []string{"s1", "s2"})
	m = press(t, m, runeKey("tab"))
	m.status = "destroyed 1 snapshot(s)"

	m = press(t, m, runeKey("j"))
	if m.status != "" {
		t.Fatalf("status not cleared on keypress")
	}
	if m.snapCur != 1 {
		t.Fatalf("keypress that cleared status was swallowed: snapCur = %d", m.snapCur)
	}
}

func TestDestroyConfirmationQueuesSelectedSnapshots(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", nil, []string{"s1", "s2"})
	m = press(t, m, runeKey("tab"))
	m = press(t, m, runeKey("x"))
	m = press(t, m, runeKey("j"))
	m = press(t, m, runeKey("x"))
	m = press(t, m, runeKey("D"))
	if !m.confirming {
		t.Fatalf("D did not open confirmation")
	}

	m = press(t, m, runeKey("n"))
	if m.confirming || len(m.destroyQ) != 0 {
		t.Fatalf("n must cancel the confirmation")
	}

	m = press(t, m, runeKey("D"))
	// Simulate the operator confirming without running the zfs command.
	m.confirming = false
	m.destroyQ = m.selectedNames()
	if got := strings.Join(m.destroyQ, ","); got != "tank/data@s1,tank/data@s2" {
		t.Fatalf("destroy queue = %q", got)
	}
}

func TestViewFitsTerminal(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", []string{"tank/data/a"}, []string{"snap-1"})
	m.pane = paneDatasets

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("view has %d lines, want %d", len(lines), m.height)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != m.width {
			t.Errorf("line %d width = %d, want %d: %q", i, got, m.width, line)
		}
	}
	if !strings.Contains(view, "▸ a") {
		t.Errorf("view does not show the child dataset:\n%s", view)
	}
	if !strings.Contains(view, "snap-1") {
		t.Errorf("view does not show the snapshot:\n%s", view)
	}
}

func TestViewTooSmall(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 20, 5
	if got := m.View(); !strings.Contains(got, "terminal too small") {
		t.Fatalf("small terminal view = %q", got)
	}
}

func TestBrowsePoolsWithoutArgument(t *testing.T) {
	m := initialModel("")
	m.width, m.height = 80, 24
	m = loadAll(t, m, "", []string{"tank", "zroot"}, nil)

	if got := m.datasetTitle(); got != "Pools (2)" {
		t.Fatalf("dataset title = %q, want Pools (2)", got)
	}
	if m.pane != paneDatasets {
		t.Fatalf("pane = %v, want datasets", m.pane)
	}

	m = press(t, m, runeKey("enter"))
	if m.current != "tank" {
		t.Fatalf("enter on pool gave current = %q, want tank", m.current)
	}

	// Inside a pool the first row is "..", which returns to the pool list.
	m = loadAll(t, m, "tank", []string{"tank/data"}, nil)
	m = press(t, m, runeKey("enter"))
	if m.current != "" {
		t.Fatalf("enter on '..' gave current = %q, want pool list", m.current)
	}

	// Descending into a pool's filesystem works too.
	m = loadAll(t, m, "", []string{"tank", "zroot"}, nil) // the root reload issued by the ascend
	m = press(t, m, runeKey("enter"))                     // back into "tank" (cursor is on the first pool)
	m = loadAll(t, m, "tank", []string{"tank/data"}, nil)
	m = press(t, m, runeKey("j")) // skip ".."
	m = press(t, m, runeKey("enter"))
	if m.current != "tank/data" {
		t.Fatalf("current = %q, want tank/data", m.current)
	}
}

func TestPoolRootView(t *testing.T) {
	m := initialModel("")
	m.width, m.height = 80, 24
	m = loadAll(t, m, "", []string{"tank", "zroot"}, nil)

	view := m.View()
	for _, want := range []string{"Pools (2)", "select a pool", "tank", "zroot"} {
		if !strings.Contains(view, want) {
			t.Errorf("pool root view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Free:") {
		t.Errorf("pool root must not claim a free-space value:\n%s", view)
	}
}

func TestLoadingShowsSpinnerNotEmptyState(t *testing.T) {
	m := initialModel("tank/data")
	m.width, m.height = 80, 24
	body := strings.Join(m.datasetLines(30, 10), "\n")
	if strings.Contains(body, "no child datasets") {
		t.Fatalf("empty state shown while loading:\n%s", body)
	}
	if !strings.Contains(body, "loading") {
		t.Fatalf("loading state missing:\n%s", body)
	}
}

func TestPoolContextRow(t *testing.T) {
	m := initialModel("tank/data")
	m = loadAll(t, m, "tank/data", []string{"tank/data/a"}, nil)
	lines := m.datasetLines(24, 8)
	if len(lines) == 0 || !strings.Contains(lines[0], "pool") || !strings.Contains(lines[0], "tank") {
		t.Fatalf("first dataset line = %q, want pool context row", strings.Join(lines[:1], ""))
	}

	// At the pool level there is no context row.
	root := initialModel("")
	root = press(t, root, datasetsMsg{dataset: "", children: datasetItems([]string{"tank"})})
	rootLines := root.datasetLines(24, 8)
	if strings.Contains(strings.Join(rootLines, "\n"), "pool ") {
		t.Fatalf("pool list must not show a pool context row:\n%s", strings.Join(rootLines, "\n"))
	}
}

func TestSpinnerTicksOnlyWhileBusy(t *testing.T) {
	idle := initialModel("tank/data")
	idle.loadingDatasets, idle.loadingSnapshots = false, false
	if _, cmd := idle.Update(spinner.TickMsg{}); cmd != nil {
		t.Fatalf("idle model requested another spinner tick")
	}

	busy := initialModel("tank/data") // loading on startup
	if _, cmd := busy.Update(spinner.TickMsg{}); cmd == nil {
		t.Fatalf("busy model stopped the spinner")
	}
}

func TestDatasetsLoadBeforeSnapshots(t *testing.T) {
	m := initialModel("tank/data")
	m.width, m.height = 80, 24

	next, cmd := m.Update(datasetsMsg{dataset: "tank/data", children: datasetItems([]string{"tank/data/a"})})
	m = next.(model)
	if cmd == nil {
		t.Fatalf("dataset result must schedule the snapshot load")
	}
	if len(m.children) != 1 {
		t.Fatalf("children = %v, want the child dataset", m.children)
	}
	if !m.loadingSnapshots {
		t.Fatalf("snapshots must still be loading after the dataset result")
	}

	view := m.View()
	if !strings.Contains(view, "▸ a") {
		t.Errorf("child dataset not rendered before snapshots:\n%s", view)
	}
	if !strings.Contains(view, "loading snapshots") {
		t.Errorf("snapshot pane does not show progress:\n%s", view)
	}
	if strings.Contains(view, "no snapshots") {
		t.Errorf("empty state shown while snapshots load:\n%s", view)
	}
}

func TestColorProfileModes(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")

	if got := colorProfile(colorAlways); got != termenv.TrueColor {
		t.Errorf("always: %v, want TrueColor", got)
	}
	if got := colorProfile(colorNever); got != termenv.Ascii {
		t.Errorf("never: %v, want Ascii", got)
	}

	t.Setenv("NO_COLOR", "1")
	if got := colorProfile(colorAuto); got != termenv.Ascii {
		t.Errorf("NO_COLOR: %v, want Ascii", got)
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	if got := colorProfile(colorAuto); got != termenv.TrueColor {
		t.Errorf("CLICOLOR_FORCE: %v, want TrueColor", got)
	}
}

func TestNavigationUsesCachedDatasets(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", []string{"tank/data/a"}, []string{"s1"})

	m = press(t, m, runeKey("enter")) // into tank/data/a, not cached yet
	m = press(t, m, datasetsMsg{dataset: "tank/data/a", children: datasetItems([]string{"tank/data/a/x"})})
	m = press(t, m, snapshotsMsg{dataset: "tank/data/a"})

	// Ascending to tank/data must use the cached children and only load snapshots.
	next, cmd := m.Update(runeKey("backspace"))
	m = next.(model)
	if m.current != "tank/data" {
		t.Fatalf("current = %q, want tank/data", m.current)
	}
	if cmd == nil {
		t.Fatalf("snapshots of the cached dataset must still load")
	}
	if m.loadingDatasets {
		t.Fatalf("cached children must not trigger a dataset reload")
	}
	if len(m.children) != 1 || m.children[0].Name != "tank/data/a" {
		t.Fatalf("children = %v, want cached tank/data/a", m.children)
	}
}

func TestRefreshDropsDatasetCache(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", []string{"tank/data/a"}, nil)
	m = press(t, m, runeKey("enter"))
	m = press(t, m, datasetsMsg{dataset: "tank/data/a"})
	m = press(t, m, snapshotsMsg{dataset: "tank/data/a"})
	m = press(t, m, runeKey("backspace"))
	if len(m.cache) == 0 {
		t.Fatalf("expected cached dataset paths")
	}

	m = press(t, m, runeKey("r"))
	if len(m.cache) != 0 {
		t.Fatalf("'r' must drop the cache, still has %d entries", len(m.cache))
	}
	if !m.loadingDatasets {
		t.Fatalf("'r' must reload the current dataset")
	}

	m = press(t, m, runeKey("enter")) // descend again: must refetch
	if !m.loadingDatasets {
		t.Fatalf("navigation after refresh must refetch children")
	}
}

func TestFreeSpaceRefreshedOnCachedNavigation(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", []string{"tank/data/a"}, nil)

	m = press(t, m, runeKey("enter"))
	m = press(t, m, datasetsMsg{dataset: "tank/data/a"})
	m = press(t, m, snapshotsMsg{dataset: "tank/data/a", avail: 555})
	if m.avail != 555 {
		t.Fatalf("avail = %d, want 555 from the snapshot result", m.avail)
	}

	// Ascending hits the children cache, but free space must not come from it.
	next, cmd := m.Update(runeKey("backspace"))
	m = next.(model)
	if cmd == nil {
		t.Fatalf("cached navigation must still load snapshots and free space")
	}
	m = press(t, m, snapshotsMsg{dataset: "tank/data", avail: 42})
	if m.avail != 42 {
		t.Fatalf("avail = %d, want refreshed 42", m.avail)
	}
}

func TestSnapshotPaneNamesDataset(t *testing.T) {
	m := newTestModel()
	m = loadAll(t, m, "tank/data", []string{"tank/data/a"}, []string{"s1", "s2"})

	view := m.View()
	if !strings.Contains(view, "Snapshots of tank/data") {
		t.Errorf("snapshot pane does not name the dataset:\n%s", view)
	}

	m = press(t, m, runeKey("tab"))
	m = press(t, m, runeKey("x"))
	m = press(t, m, runeKey("D"))
	if got := m.View(); !strings.Contains(got, "in tank/data?") {
		t.Errorf("destroy prompt does not name the dataset:\n%s", got)
	}
}
