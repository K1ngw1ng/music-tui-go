package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type trackItem struct{ t Track }

func (i trackItem) Title() string       { return i.t.DisplayTitle() }
func (i trackItem) Description() string { return i.t.DisplayArtist() }
func (i trackItem) FilterValue() string { return i.t.DisplayTitle() + " " + i.t.DisplayArtist() }

type trackDoneMsg struct{ idx int }
type artReadyMsg struct{ art string }
type playlistSavedMsg struct{ err error }

var (
	artPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	listPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

type model struct {
	tracks  []Track
	list    list.Model
	player  *Player
	current int
	art     string
	width   int
	height  int
	loading bool
	title   string
	cfg     Config

	prompting    bool
	promptInput  textinput.Model
	promptTrack  int
	promptErr    string
	promptOk     string
}

func newModel(tracks []Track, cfg Config, title string) model {
	items := make([]list.Item, len(tracks))
	for i, t := range tracks {
		items[i] = trackItem{t}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Padding(0, 1)

	ti := textinput.New()
	ti.Placeholder = "playlist name"
	ti.CharLimit = 64

	return model{
		tracks:      tracks,
		list:        l,
		player:      &Player{},
		current:     -1,
		title:       title,
		cfg:         cfg,
		promptInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	if len(m.tracks) > 0 {
		artW, artH := m.artDims()
		if artW > 0 {
			return loadArt(m.tracks[0].Path, artW, artH)
		}
	}
	return nil
}

func loadArt(path string, w, h int) tea.Cmd {
	return func() tea.Msg {
		art := renderArt(path, w, h)
		return artReadyMsg{art: art}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.prompting {
		return m.updatePrompt(msg)
	}

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.relayout()
		if m.art == "" && !m.loading && len(m.tracks) > 0 {
			m.loading = true
			idx := m.list.Index()
			artW, artH := m.artDims()
			return m, loadArt(m.tracks[idx].Path, artW, artH)
		}
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.player.Stop()
			return m, tea.Quit
		case "enter", " ":
			sel := m.list.Index()
			if sel == m.current {
				m.player.TogglePause()
			} else {
				return m, m.playTrack(sel)
			}
			return m, nil
		case "n", "right":
			return m, m.playTrack(m.nextIndex())
		case "p", "left":
			return m, m.playTrack(m.prevIndex())
		case "s":
			m.player.Stop()
			m.current = -1
			m.art = ""
			return m, nil
		case "a":
			return m.startPrompt(m.list.Index()), nil
		}

	case trackDoneMsg:
		if msg.idx == m.current {
			return m, m.playTrack(m.nextIndex())
		}
		return m, nil

	case artReadyMsg:
		m.art = msg.art
		m.loading = false
		return m, nil

	case playlistSavedMsg:
		if msg.err != nil {
			m.promptErr = msg.err.Error()
		}
		return m, nil
	}

	var cmd tea.Cmd
	prevIdx := m.list.Index()
	m.list, cmd = m.list.Update(msg)
	newIdx := m.list.Index()
	if newIdx != prevIdx && m.current == -1 {
		artW, artH := m.artDims()
		return m, tea.Batch(cmd, loadArt(m.tracks[newIdx].Path, artW, artH))
	}
	return m, cmd
}

func (m model) startPrompt(trackIdx int) model {
	m.prompting = true
	m.promptTrack = trackIdx
	m.promptErr = ""
	m.promptOk = ""
	m.promptInput.SetValue("")
	m.promptInput.Focus()
	return m
}

func (m model) updatePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.prompting = false
			m.promptInput.Blur()
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.promptInput.Value())
			if name == "" {
				m.promptErr = "playlist name cannot be empty"
				return m, nil
			}
			m3uPath, err := resolvePlaylist(m.cfg, name)
			if err != nil {
				m.promptErr = fmt.Sprintf("unknown playlist %q — create it with --create-playlist first", name)
				return m, nil
			}
			track := m.tracks[m.promptTrack]
			m.prompting = false
			m.promptInput.Blur()
			m.promptOk = fmt.Sprintf("Added %q to %q", track.DisplayTitle(), name)
			return m, saveToPlaylist(m3uPath, track)
		}
	}
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

func saveToPlaylist(m3uPath string, track Track) tea.Cmd {
	return func() tea.Msg {
		err := appendM3U(m3uPath, []Track{track})
		return playlistSavedMsg{err: err}
	}
}

func (m *model) playTrack(idx int) tea.Cmd {
	if len(m.tracks) == 0 || idx < 0 {
		return nil
	}
	m.current = idx
	m.list.Select(idx)
	m.art = ""
	m.loading = true
	track := m.tracks[idx]

	doneCh := make(chan int, 1)
	m.player.Play(track.Path, func() {
		doneCh <- idx
	})

	artW, artH := m.artDims()
	return tea.Batch(
		loadArt(track.Path, artW, artH),
		waitForDone(doneCh),
	)
}

func waitForDone(ch chan int) tea.Cmd {
	return func() tea.Msg {
		return trackDoneMsg{idx: <-ch}
	}
}

func (m *model) nextIndex() int {
	if len(m.tracks) == 0 {
		return -1
	}
	return (m.current + 1) % len(m.tracks)
}

func (m *model) prevIndex() int {
	if len(m.tracks) == 0 {
		return -1
	}
	if m.current <= 0 {
		return len(m.tracks) - 1
	}
	return m.current - 1
}

func (m *model) artDims() (w, h int) {
	panelW := m.width/2 - 4
	panelH := m.height - 8
	if panelW < 1 {
		panelW = 20
	}
	if panelH < 1 {
		panelH = 10
	}

	charW := panelW
	charH := charW / 2
	if charH > panelH {
		charH = panelH
		charW = charH * 2
	}
	return charW, charH
}

func (m *model) relayout() {
	listW := m.width/2 - 4
	listH := m.height - 8
	if listW < 10 {
		listW = 10
	}
	if listH < 4 {
		listH = 4
	}
	m.list.SetSize(listW, listH)
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	panelH := m.height - 6
	panelInnerW := m.width/2 - 4

	artContent := m.art
	if m.loading {
		artContent = centered("⟳ Loading art...", panelInnerW, panelH)
	} else if artContent == "" {
		artContent = noArtPlaceholder(panelInnerW, panelH)
	}
	leftPanel := artPanelStyle.
		Width(m.width/2 - 2).
		Height(panelH).
		Render(artContent)

	rightPanel := listPanelStyle.
		Width(m.width - m.width/2 - 2).
		Height(panelH).
		Render(m.list.View())

	top := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	status := m.statusLine()

	var bottom string
	if m.prompting {
		bottom = promptStyle.Render("Add to playlist: ") + m.promptInput.View()
		if m.promptErr != "" {
			bottom += "  " + errStyle.Render(m.promptErr)
		}
		bottom += "\n" + helpStyle.Render("enter: confirm  esc: cancel")
	} else {
		msg := ""
		if m.promptOk != "" {
			msg = "  " + helpStyle.Render("✓ "+m.promptOk)
		} else if m.promptErr != "" {
			msg = "  " + errStyle.Render(m.promptErr)
		}
		bottom = helpStyle.Render("enter/space: play/pause  n/→: next  p/←: prev  s: stop  a: add to playlist  /: filter  q: quit") + msg
	}

	return lipgloss.JoinVertical(lipgloss.Left, top, status, bottom)
}

func (m model) statusLine() string {
	if m.current < 0 || m.current >= len(m.tracks) {
		return statusStyle.Render("  ▶  No track selected")
	}
	t := m.tracks[m.current]
	state := "▶"
	if m.player.IsPaused() {
		state = "⏸"
	}
	title := t.DisplayTitle()
	artist := t.DisplayArtist()
	album := t.Album
	if album == "" {
		album = filepath.Base(filepath.Dir(t.Path))
	}
	line := fmt.Sprintf("  %s  %s — %s  [%s]", state, title, artist, album)
	return statusStyle.Render(line)
}

func centered(s string, w, h int) string {
	lines := make([]string, h)
	mid := h / 2
	pad := (w - len(s)) / 2
	if pad < 0 {
		pad = 0
	}
	for i := range lines {
		if i == mid {
			lines[i] = strings.Repeat(" ", pad) + s
		} else {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}