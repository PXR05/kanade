package tui

import (
	"fmt"
	"gmp/audio"
	lib "gmp/library"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	LibraryView ViewState = iota
	PlayerView
)

type Model struct {
	currentView ViewState
	width       int
	height      int

	libraryModel *LibraryModel
	playerModel  *PlayerModel

	library          *lib.Library
	audioPlayer      *audio.Player
	songs            []lib.Song
	currentSongIndex int

	selectedSong *lib.Song

	lastError    error
	errorTimeout time.Time
}

type (
	SongSelectedMsg struct {
		Song lib.Song
	}

	NextTrackMsg    struct{}
	PrevTrackMsg    struct{}
	SongFinishedMsg struct{}

	SwitchViewMsg struct {
		View ViewState
	}

	PlaybackStatusMsg struct {
		IsPlaying bool
		Position  string
		Error     error
	}

	PlaybackPositionMsg struct {
		Position      time.Duration
		TotalDuration time.Duration
	}

	WindowSizeMsg struct {
		Width, Height int
	}

	TickMsg struct {
		Time time.Time
	}

	ErrorMsg struct {
		Error error
	}
)

func NewModel(library *lib.Library, audioPlayer *audio.Player) *Model {
	songs := library.ListSongs()

	libraryModel := NewLibraryModel(songs)
	playerModel := NewPlayerModel(audioPlayer)

	model := &Model{
		currentView:      LibraryView,
		library:          library,
		audioPlayer:      audioPlayer,
		songs:            songs,
		currentSongIndex: -1,
		libraryModel:     libraryModel,
		playerModel:      playerModel,
	}

	audioPlayer.SetErrorCallback(func(err error) {
		log.Printf("Audio player error: %v", err)
	})

	return model
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.libraryModel.Init(),
		m.playerModel.Init(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		libraryModel, libraryCmd := m.libraryModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.libraryModel = libraryModel.(*LibraryModel)

		playerModel, playerCmd := m.playerModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.playerModel = playerModel.(*PlayerModel)

		cmds = append(cmds, libraryCmd, playerCmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.currentView == PlayerView {
				m.currentView = LibraryView
				return m, nil
			}

			if m.currentView == LibraryView && m.libraryModel.IsInSearchMode() {
				break
			}
			return m, tea.Quit
		case "esc":
			if m.currentView == PlayerView {
				m.currentView = LibraryView
				return m, nil
			}
		}

	case ErrorMsg:
		m.lastError = msg.Error
		m.errorTimeout = time.Now().Add(ErrorTimeout)

		playerModel, playerCmd := m.playerModel.Update(PlaybackStatusMsg{
			Error: msg.Error,
		})
		m.playerModel = playerModel.(*PlayerModel)
		cmds = append(cmds, playerCmd)

	case NextTrackMsg, SongFinishedMsg:
		return m, m.playNextTrack()

	case PrevTrackMsg:
		return m, m.playPreviousTrack()

	case SongSelectedMsg:
		return m.handleSongSelection(msg)

	case SwitchViewMsg:
		m.currentView = msg.View

		return m, nil
	}

	if tickMsg, ok := msg.(TickMsg); ok {
		if m.lastError != nil && time.Now().After(m.errorTimeout) {
			m.lastError = nil
		}

		if m.audioPlayer.HasPlaybackFinished() || m.audioPlayer.IsAtEnd() {
			cmds = append(cmds, func() tea.Msg {
				return SongFinishedMsg{}
			})
		}

		statusMsg := PlaybackStatusMsg{
			IsPlaying: m.audioPlayer.IsPlaying(),
		}

		libraryModel, _ := m.libraryModel.Update(statusMsg)
		m.libraryModel = libraryModel.(*LibraryModel)

		if m.currentView == LibraryView {
			positionMsg := PlaybackPositionMsg{
				Position:      m.audioPlayer.GetPlaybackPosition(),
				TotalDuration: m.audioPlayer.GetTotalLength(),
			}
			libraryModel, _ = m.libraryModel.Update(positionMsg)
			m.libraryModel = libraryModel.(*LibraryModel)
		}

		if err := m.audioPlayer.GetLastError(); err != nil && err != m.lastError {
			cmds = append(cmds, func() tea.Msg {
				return ErrorMsg{Error: err}
			})
		}

		_ = tickMsg
	}

	switch m.currentView {
	case LibraryView:
		libraryModel, cmd := m.libraryModel.Update(msg)
		m.libraryModel = libraryModel.(*LibraryModel)
		cmds = append(cmds, cmd)

	case PlayerView:

		playerModel, cmd := m.playerModel.Update(msg)
		m.playerModel = playerModel.(*PlayerModel)
		cmds = append(cmds, cmd)

		if _, ok := msg.(PlaybackStatusMsg); ok {
			libraryModel, _ := m.libraryModel.Update(msg)
			m.libraryModel = libraryModel.(*LibraryModel)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleSongSelection(msg SongSelectedMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	m.currentView = PlayerView

	m.currentSongIndex = m.libraryModel.FindSongIndex(msg.Song)

	libraryModel, _ := m.libraryModel.Update(msg)
	m.libraryModel = libraryModel.(*LibraryModel)

	if m.selectedSong != nil && m.selectedSong.Path == msg.Song.Path {
		playerModel, playerCmd := m.playerModel.Update(msg)
		m.playerModel = playerModel.(*PlayerModel)
		return m, playerCmd
	}

	m.selectedSong = &msg.Song

	if err := m.loadAndPlaySong(msg.Song); err != nil {
		playerModel, playerCmd := m.playerModel.Update(PlaybackStatusMsg{
			Error: err,
		})
		m.playerModel = playerModel.(*PlayerModel)
		cmds = append(cmds, playerCmd)

		cmds = append(cmds, func() tea.Msg {
			return ErrorMsg{Error: err}
		})
	} else {
		playerModel, playerCmd := m.playerModel.Update(msg)
		m.playerModel = playerModel.(*PlayerModel)
		cmds = append(cmds, playerCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) loadAndPlaySong(song lib.Song) error {

	if m.audioPlayer.IsPlaying() {
		m.audioPlayer.Stop()
	}

	if song.Path == "" {
		return fmt.Errorf("invalid song path")
	}

	if err := m.audioPlayer.Load(song.Path); err != nil {
		return fmt.Errorf("failed to load song '%s': %w", song.Title, err)
	}

	if err := m.audioPlayer.Play(); err != nil {
		return fmt.Errorf("failed to play song '%s': %w", song.Title, err)
	}

	m.audioPlayer.ForceGC()

	return nil
}

func (m *Model) GetLastError() error {
	return m.lastError
}

func (m *Model) View() string {
	switch m.currentView {
	case LibraryView:
		return m.libraryModel.View()
	case PlayerView:
		return m.playerModel.View()
	default:
		return "Unknown view"
	}
}

func (m *Model) playNextTrack() tea.Cmd {

	orderedSongs := m.libraryModel.GetOrderedSongs()

	if len(orderedSongs) == 0 || m.currentSongIndex < 0 {
		return nil
	}

	nextIndex := m.currentSongIndex + 1
	if nextIndex >= len(orderedSongs) {

		nextIndex = 0
	}

	m.currentSongIndex = nextIndex
	nextSong := orderedSongs[nextIndex]

	return func() tea.Msg {
		return SongSelectedMsg{Song: nextSong}
	}
}

func (m *Model) playPreviousTrack() tea.Cmd {

	orderedSongs := m.libraryModel.GetOrderedSongs()

	if len(orderedSongs) == 0 || m.currentSongIndex < 0 {
		return nil
	}

	prevIndex := m.currentSongIndex - 1
	if prevIndex < 0 {

		prevIndex = len(orderedSongs) - 1
	}

	m.currentSongIndex = prevIndex
	prevSong := orderedSongs[prevIndex]

	return func() tea.Msg {
		return SongSelectedMsg{Song: prevSong}
	}
}
