package tui

import (
	"log"
	"strings"
	"time"

	"kanade/audio"
	lib "kanade/library"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	previousView ViewState
	currentView  ViewState
	width        int
	height       int

	libraryModel *LibraryModel
	playerModel  *PlayerModel

	library     *lib.Library
	AudioPlayer *audio.Player
	libraryDir  string

	currentSongIndex int
	SelectedSong     *lib.Song

	dominantColor    string
	coloredSongPath  string
	albumArtRenderer *AlbumArtRenderer

	commandBar *CommandBar

	repeatMode RepeatMode
	shuffle    bool

	volume     float64
	muted      bool
	preMuteVol float64

	statusText    string
	statusIsError bool
	statusExpiry  time.Time

	helpOpen bool

	sendFn func(tea.Msg)

	finishHandled bool

	lastError error
}

func NewModel(library *lib.Library, audioPlayer *audio.Player, dir string) *Model {
	songs := library.ListSongs()

	model := &Model{
		previousView:     LibraryView,
		currentView:      LibraryView,
		library:          library,
		AudioPlayer:      audioPlayer,
		libraryDir:       dir,
		currentSongIndex: -1,
		libraryModel:     NewLibraryModel(songs),
		playerModel:      NewPlayerModel(audioPlayer),
		dominantColor:    DefaultAccentColor,
		albumArtRenderer: NewAlbumArtRenderer(AlbumArtMinMax, AlbumArtMinMax),
		commandBar:       NewCommandBar(),
		repeatMode:       RepeatOff,
		volume:           audioPlayer.GetVolume(),
		preMuteVol:       audio.DefaultVolume,
	}

	audioPlayer.SetErrorCallback(func(err error) {
		log.Printf("Audio player error: %v", err)
	})

	return model
}

func (m *Model) SetProgram(p func(tea.Msg)) {
	m.sendFn = p
}

func (m *Model) Init() tea.Cmd {
	return m.scheduleTick()
}

func (m *Model) scheduleTick() tea.Cmd {
	return tea.Tick(TickInterval, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case TickMsg:
		return m.handleTick()

	case ErrorMsg:
		if msg.Error != nil {
			log.Printf("Error: %v", msg.Error)
			m.lastError = msg.Error
			return m, m.showToast(msg.Error.Error(), true)
		}
		return m, nil

	case StatusMsg:
		m.statusText = msg.Text
		m.statusIsError = msg.IsError
		m.statusExpiry = time.Now().Add(StatusTimeout)
		lm, _ := m.libraryModel.Update(msg)
		m.libraryModel = lm.(*LibraryModel)
		pm, _ := m.playerModel.Update(msg)
		m.playerModel = pm.(*PlayerModel)
		return m, nil

	case SongSelectedMsg:
		m.selectSong(msg.Song, msg.KeepView)
		return m, nil

	case NextTrackMsg:
		return m, m.nextTrack(true)

	case PrevTrackMsg:
		return m, m.prevTrack()

	case SongFinishedMsg:
		return m, m.handleSongFinished()

	case SwitchViewMsg:
		m.switchView(msg.View)
		return m, nil

	case PlayMsg:
		return m, m.play()
	case PauseMsg:
		return m, m.pause()
	case TogglePlayMsg:
		return m, m.togglePlay()
	case StopRequestMsg:
		return m, m.stopPlayback()
	}

	return m, m.forwardToCurrentView(msg)
}

func (m *Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.albumArtRenderer = NewResponsiveAlbumArtRenderer(m.width, m.height)
	m.commandBar.UpdateSize(m.width, m.height)

	var cmds []tea.Cmd
	for _, sub := range []tea.Model{m.libraryModel, m.playerModel} {
		updated, cmd := sub.Update(msg)
		cmds = append(cmds, cmd)
		switch v := updated.(type) {
		case *LibraryModel:
			m.libraryModel = v
		case *PlayerModel:
			m.playerModel = v
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.helpOpen {
		m.helpOpen = false
		return m, nil
	}

	if m.commandBar.Active {
		return m.handleCommandBarKey(msg, key)
	}

	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "?":
		m.helpOpen = true
		return m, nil

	case "esc":
		if m.currentView == PlayerView {
			m.switchView(LibraryView)
		}
		return m, nil

	case "tab":
		if m.currentView != PlayerView {
			m.previousView = m.currentView
			m.switchView(PlayerView)
		} else {
			m.switchView(m.previousView)
		}
		return m, nil

	case "p":
		return m, m.togglePlay()

	case "+", "=":
		return m, m.changeVolume(VolumeStep)
	case "-", "_":
		return m, m.changeVolume(-VolumeStep)
	case "m":
		return m, m.toggleMute()

	case "up", "down":

		if m.currentView == PlayerView {
			delta := VolumeStep
			if key == "down" {
				delta = -VolumeStep
			}
			return m, m.changeVolume(delta)
		}

	case "r":
		return m, m.cycleRepeatMode()
	case "z":
		return m, m.toggleShuffle()

	case "R":
		if m.currentView == LibraryView {
			return m, m.rescanLibrary()
		}
		return m, nil

	case "/":
		if m.currentView == LibraryView {
			m.commandBar.Active = true
			m.commandBar.Prompt = "/"
			m.commandBar.Reset()
			m.libraryModel.searchQuery = ""
			m.libraryModel.filterSongs()
		}
		return m, nil

	case ":":
		m.commandBar.Active = true
		m.commandBar.Prompt = ":"
		m.commandBar.Reset()
		return m, nil
	}

	return m, m.forwardToCurrentView(msg)
}

func (m *Model) handleCommandBarKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	isSearch := m.commandBar.Prompt == "/" && m.currentView == LibraryView

	switch key {
	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		if isSearch {
			m.commandBar.Active = false
			m.commandBar.Reset()
			return m, nil
		}
		input := strings.TrimSpace(m.commandBar.Input)
		m.commandBar.Active = false
		m.commandBar.Reset()
		if input != "" {
			return m, m.executeCommand(input)
		}
		return m, nil

	case "esc":
		if isSearch {
			m.libraryModel.searchQuery = ""
			m.libraryModel.filterSongs()
		}
		m.commandBar.Active = false
		m.commandBar.Reset()
		return m, nil

	case "backspace":
		if len(m.commandBar.Input) > 0 {
			m.commandBar.Input = m.commandBar.Input[:len(m.commandBar.Input)-1]
		}
		if isSearch {
			m.libraryModel.searchQuery = m.commandBar.Input
			m.libraryModel.filterSongs()
		}
		return m, nil

	default:
		if updated := AppendRunesInput(m.commandBar.Input, msg); updated != m.commandBar.Input {
			m.commandBar.Input = updated
			if isSearch {
				m.libraryModel.searchQuery = m.commandBar.Input
				m.libraryModel.filterSongs()
			}
		}
		return m, nil
	}
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if m.currentView == PlayerView {
			delta := VolumeStep
			if msg.Button == tea.MouseButtonWheelDown {
				delta = -VolumeStep
			}
			return m, m.changeVolume(delta)
		}
		keyType := tea.KeyUp
		if msg.Button == tea.MouseButtonWheelDown {
			keyType = tea.KeyDown
		}
		lm, _ := m.libraryModel.Update(tea.KeyMsg{Type: keyType})
		m.libraryModel = lm.(*LibraryModel)
		return m, nil

	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch m.currentView {
		case LibraryView:
			return m, m.libraryModel.HandleClick(msg)
		case PlayerView:
			return m, m.playerModel.HandleClick(msg)
		}
	}
	return m, nil
}

func (m *Model) handleTick() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.scheduleTick()}

	m.expireStatus()

	if m.AudioPlayer != nil {
		finished := m.AudioPlayer.ConsumePlaybackFinished()
		if !finished && !m.finishHandled && m.AudioPlayer.IsPlaying() && m.AudioPlayer.IsAtEnd() {
			finished = true
		}
		if finished && !m.finishHandled {
			m.finishHandled = true
			cmds = append(cmds, func() tea.Msg { return SongFinishedMsg{} })
		}

		status := PlaybackStatusMsg{IsPlaying: m.AudioPlayer.IsPlaying()}
		position := PlaybackPositionMsg{
			Position:      m.AudioPlayer.GetPlaybackPosition(),
			TotalDuration: m.AudioPlayer.GetTotalLength(),
		}

		lm, _ := m.libraryModel.Update(position)
		m.libraryModel = lm.(*LibraryModel)

		pm, _ := m.playerModel.Update(status)
		m.playerModel = pm.(*PlayerModel)
		pm, _ = m.playerModel.Update(position)
		m.playerModel = pm.(*PlayerModel)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) forwardToCurrentView(msg tea.Msg) tea.Cmd {
	switch m.currentView {
	case LibraryView:
		updated, cmd := m.libraryModel.Update(msg)
		m.libraryModel = updated.(*LibraryModel)
		return cmd
	default:
		updated, cmd := m.playerModel.Update(msg)
		m.playerModel = updated.(*PlayerModel)
		return cmd
	}
}

func (m *Model) switchView(view ViewState) {
	if view == m.currentView {
		return
	}
	if view != PlayerView {
		m.previousView = m.currentView
	}
	m.currentView = view
}

func (m *Model) expireStatus() {
	if m.statusText != "" && time.Now().After(m.statusExpiry) {
		m.statusText = ""
	}
}

func (m *Model) showToast(text string, isError bool) tea.Cmd {
	return func() tea.Msg { return StatusMsg{Text: text, IsError: isError} }
}

func (m *Model) View() string {
	if m.helpOpen {
		return RenderHelpOverlay(m.width, m.height)
	}

	var base string
	switch m.currentView {
	case LibraryView:
		base = m.libraryModel.View()
	default:
		base = m.playerModel.View()
	}

	if m.commandBar.Active {
		if !strings.HasSuffix(base, "\n") {
			base += "\n"
		}
		base += m.commandBar.View()
	}
	return base
}

func (m *Model) GetLastError() error {
	return m.lastError
}
