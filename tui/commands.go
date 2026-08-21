package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) executeCommand(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	if strings.HasPrefix(input, "/") {
		query := strings.TrimSpace(input[1:])
		if query == "" {
			return nil
		}
		m.applySearch(query)
		return nil
	}

	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "q", "quit", "exit":
		return tea.Quit

	case "play":
		return m.play()
	case "pause":
		return m.pause()
	case "next":
		return m.nextTrack(true)
	case "prev", "previous":
		return m.prevTrack()
	case "stop":
		return m.stopPlayback()

	case "vol", "volume":
		return m.commandVolume(args)

	case "mute":
		return m.toggleMute()

	case "shuffle":
		return m.commandShuffle(args)

	case "repeat":
		return m.commandRepeat(args)

	case "search", "s":
		if len(args) == 0 {
			return m.showToast("Usage: :search <query>", true)
		}
		m.applySearch(strings.Join(args, " "))
		return nil

	case "view", "v":
		return m.commandView(args)

	case "vl":
		return func() tea.Msg { return SwitchViewMsg{View: LibraryView} }
	case "vp":
		return func() tea.Msg { return SwitchViewMsg{View: PlayerView} }

	case "reload", "rescan", "refresh":
		return m.rescanLibrary()

	case "help", "h", "?":
		m.helpOpen = true
		return nil
	}

	return m.showToast(fmt.Sprintf("Unknown command '%s' — try :help", cmd), true)
}

func (m *Model) applySearch(query string) {
	m.currentView = LibraryView
	m.libraryModel.searchQuery = query
	m.libraryModel.filterSongs()
}

func (m *Model) commandVolume(args []string) tea.Cmd {
	if len(args) != 1 {
		return m.showToast("Usage: :vol <0-100>", true)
	}
	pct, err := strconv.Atoi(strings.TrimSuffix(args[0], "%"))
	if err != nil || pct < 0 || pct > 100 {
		return m.showToast("Volume must be 0-100", true)
	}
	return m.setVolume(float64(pct) / 100.0)
}

func (m *Model) commandShuffle(args []string) tea.Cmd {
	if len(args) > 0 {
		want := strings.ToLower(args[0]) == "on"
		if want != m.shuffle {
			return m.toggleShuffle()
		}
		return nil
	}
	return m.toggleShuffle()
}

func (m *Model) commandRepeat(args []string) tea.Cmd {
	if len(args) == 0 {
		return m.cycleRepeatMode()
	}
	var target RepeatMode
	switch strings.ToLower(args[0]) {
	case "off", "none":
		target = RepeatOff
	case "all", "queue":
		target = RepeatAll
	case "one", "track":
		target = RepeatOne
	default:
		return m.showToast("Usage: :repeat [off|all|one]", true)
	}
	if target == m.repeatMode {
		return nil
	}
	m.repeatMode = target
	return m.showToast(target.Label(), false)
}

func (m *Model) commandView(args []string) tea.Cmd {
	if len(args) < 1 {
		return m.showToast("Usage: :view <library|player>", true)
	}
	switch strings.ToLower(args[0]) {
	case "lib", "library", "l":
		return func() tea.Msg { return SwitchViewMsg{View: LibraryView} }
	case "player", "p":
		return func() tea.Msg { return SwitchViewMsg{View: PlayerView} }
	default:
		return m.showToast(fmt.Sprintf("Unknown view '%s'", args[0]), true)
	}
}
