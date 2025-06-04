package tui

import (
	"gmp/audio"
	lib "gmp/library"

	tea "github.com/charmbracelet/bubbletea"
)

// ViewState represents the current view in the application
type ViewState int

const (
	LibraryView ViewState = iota
	PlayerView
)

// Model represents the main application model
type Model struct {
	// State management
	currentView ViewState
	width       int
	height      int

	// Components
	libraryModel *LibraryModel
	playerModel  *PlayerModel

	// Data
	library     *lib.Library
	audioPlayer *audio.Player
	songs       []lib.Song

	// Current selection
	selectedSong *lib.Song
}

// Messages for communication between views
type (
	// SongSelectedMsg is sent when a song is selected in library view
	SongSelectedMsg struct {
		Song lib.Song
	}

	// SwitchViewMsg is sent to switch between views
	SwitchViewMsg struct {
		View ViewState
	}

	// PlaybackStatusMsg updates playback status
	PlaybackStatusMsg struct {
		IsPlaying bool
		Position  string
		Error     error
	}

	// WindowSizeMsg updates window dimensions
	WindowSizeMsg struct {
		Width, Height int
	}
)

// NewModel creates a new main model
func NewModel(library *lib.Library, audioPlayer *audio.Player) *Model {
	songs := library.ListSongs()

	libraryModel := NewLibraryModel(songs)
	playerModel := NewPlayerModel(audioPlayer)

	return &Model{
		currentView:  LibraryView,
		library:      library,
		audioPlayer:  audioPlayer,
		songs:        songs,
		libraryModel: libraryModel,
		playerModel:  playerModel,
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.libraryModel.Init(),
		m.playerModel.Init(),
	)
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update child models with new dimensions
		libraryModel, libraryCmd := m.libraryModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.libraryModel = libraryModel.(*LibraryModel)

		playerModel, playerCmd := m.playerModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.playerModel = playerModel.(*PlayerModel)

		cmds = append(cmds, libraryCmd, playerCmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.currentView == PlayerView {
				// If in player view, go back to library view
				m.currentView = LibraryView
				return m, nil
			}
			// If in library view, quit the application
			return m, tea.Quit
		case "esc":
			// Always go back to library view on escape
			if m.currentView == PlayerView {
				m.currentView = LibraryView
				return m, nil
			}
		}

	case SongSelectedMsg:
		// Song was selected in library view, switch to player view
		m.selectedSong = &msg.Song
		m.currentView = PlayerView

		// Load the song in the audio player
		err := m.audioPlayer.Load(msg.Song.Path)
		if err != nil {
			// Send error to player view
			playerModel, playerCmd := m.playerModel.Update(PlaybackStatusMsg{
				Error: err,
			})
			m.playerModel = playerModel.(*PlayerModel)
			return m, playerCmd
		}

		// Update player view with selected song
		playerModel, playerCmd := m.playerModel.Update(msg)
		m.playerModel = playerModel.(*PlayerModel)
		return m, playerCmd

	case SwitchViewMsg:
		m.currentView = msg.View
		return m, nil
	}

	// Update the current view
	switch m.currentView {
	case LibraryView:
		libraryModel, cmd := m.libraryModel.Update(msg)
		m.libraryModel = libraryModel.(*LibraryModel)
		cmds = append(cmds, cmd)

	case PlayerView:
		playerModel, cmd := m.playerModel.Update(msg)
		m.playerModel = playerModel.(*PlayerModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the current view
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
