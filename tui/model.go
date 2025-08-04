package tui

import (
	"fmt"
	"kanade/audio"
	"kanade/downloader"
	lib "kanade/library"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ViewState int

const (
	LibraryView ViewState = iota
	PlayerView
	DownloaderView
)

type Model struct {
	previousView ViewState
	currentView  ViewState
	width        int
	height       int

	libraryModel    *LibraryModel
	playerModel     *PlayerModel
	downloaderModel *DownloaderModel

	library           *lib.Library
	AudioPlayer       *audio.Player
	downloaderManager *downloader.DownloadManager
	songs             []lib.Song
	currentSongIndex  int

	SelectedSong     *lib.Song
	dominantColor    string
	albumArtRenderer *AlbumArtRenderer

	lastError    error
	errorTimeout time.Time
}

type (
	SongSelectedMsg struct {
		Song     lib.Song
		KeepView bool
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

	DownloadProgressMsg struct {
		Update downloader.ProgressUpdate
	}

	DownloadCompletedMsg struct {
		Event downloader.CompletionEvent
	}

	DownloadAddedMsg struct {
		ID  string
		URL string
	}

	DominantColorMsg struct {
		Color string
	}

	PlayPauseMsg struct{}

	StopMsg struct{}
)

func NewModel(library *lib.Library, audioPlayer *audio.Player, downloaderManager *downloader.DownloadManager) *Model {
	songs := library.ListSongs()

	libraryModel := NewLibraryModel(songs)
	playerModel := NewPlayerModel(audioPlayer)
	downloaderModel := NewDownloaderModel()
	downloaderModel.SetDownloaderManager(downloaderManager)

	model := &Model{
		previousView:      LibraryView,
		currentView:       LibraryView,
		library:           library,
		AudioPlayer:       audioPlayer,
		downloaderManager: downloaderManager,
		songs:             songs,
		currentSongIndex:  -1,
		libraryModel:      libraryModel,
		playerModel:       playerModel,
		downloaderModel:   downloaderModel,
		dominantColor:     DefaultAccentColor,
		albumArtRenderer:  NewAlbumArtRenderer(AlbumArtMinMax, AlbumArtMinMax),
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
		m.downloaderModel.Init(),
		m.listenForDownloadProgress(),
		m.listenForDownloadCompletion(),
	)
}

func (m *Model) listenForDownloadProgress() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		select {
		case update := <-m.downloaderManager.GetProgressChannel():
			return DownloadProgressMsg{Update: update}
		default:
			return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
				return m.listenForDownloadProgress()()
			})()
		}
	})
}

func (m *Model) listenForDownloadCompletion() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		select {
		case event := <-m.downloaderManager.GetCompletionChannel():
			return DownloadCompletedMsg{Event: event}
		default:
			return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
				return m.listenForDownloadCompletion()()
			})()
		}
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.albumArtRenderer = NewResponsiveAlbumArtRenderer(m.width, m.height)

		libraryModel, libraryCmd := m.libraryModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.libraryModel = libraryModel.(*LibraryModel)

		playerModel, playerCmd := m.playerModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.playerModel = playerModel.(*PlayerModel)

		downloaderModel, downloaderCmd := m.downloaderModel.Update(WindowSizeMsg{Width: msg.Width, Height: msg.Height})
		m.downloaderModel = downloaderModel.(*DownloaderModel)

		cmds = append(cmds, libraryCmd, playerCmd, downloaderCmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.currentView == PlayerView {
				m.currentView = LibraryView
				return m, nil
			}

			if m.currentView == LibraryView && m.libraryModel.searchMode {
				break
			}

			if m.currentView == DownloaderView && m.downloaderModel.inputMode {
				break
			}
			return m, tea.Quit

		case "esc":
			if m.currentView == PlayerView {
				m.currentView = LibraryView
				return m, nil
			}

		case "tab":
			if m.currentView != PlayerView {
				m.previousView = m.currentView
				return m, func() tea.Msg {
					return SwitchViewMsg{View: PlayerView}
				}
			} else {
				return m, func() tea.Msg {
					return SwitchViewMsg{View: m.previousView}
				}
			}

		case "alt+!":
			return m, func() tea.Msg {
				return SwitchViewMsg{View: LibraryView}
			}

		case "alt+@":
			return m, func() tea.Msg {
				return SwitchViewMsg{View: PlayerView}
			}

		case "alt+#":
			return m, func() tea.Msg {
				return SwitchViewMsg{View: DownloaderView}
			}

		case "p":
			if !m.libraryModel.searchMode {
				return m, m.playPause()
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
		downloaderModel, _ := m.downloaderModel.Update(msg)
		m.downloaderModel = downloaderModel.(*DownloaderModel)
		return m.handleSongSelection(msg)

	case SwitchViewMsg:
		m.currentView = msg.View
		return m, nil

	case DownloadProgressMsg:
		downloaderModel, cmd := m.downloaderModel.Update(msg)
		m.downloaderModel = downloaderModel.(*DownloaderModel)
		cmds = append(cmds, cmd)

		cmds = append(cmds, m.listenForDownloadProgress())

	case DownloadCompletedMsg:
		if msg.Event.Error == nil && msg.Event.Song != nil {
			m.songs = m.library.ListSongs()
			libraryModel := NewLibraryModel(m.songs)
			m.libraryModel = libraryModel
		}

		downloaderModel, cmd := m.downloaderModel.Update(msg)
		m.downloaderModel = downloaderModel.(*DownloaderModel)
		cmds = append(cmds, cmd)

		cmds = append(cmds, m.listenForDownloadCompletion())

	case DownloadAddedMsg:
		downloaderModel, cmd := m.downloaderModel.Update(msg)
		m.downloaderModel = downloaderModel.(*DownloaderModel)
		cmds = append(cmds, cmd)
	}

	if tickMsg, ok := msg.(TickMsg); ok {
		if m.lastError != nil && time.Now().After(m.errorTimeout) {
			m.lastError = nil
		}

		if m.AudioPlayer.HasPlaybackFinished() || m.AudioPlayer.IsAtEnd() {
			cmds = append(cmds, func() tea.Msg {
				return SongFinishedMsg{}
			})
		}

		statusMsg := PlaybackStatusMsg{
			IsPlaying: m.AudioPlayer.IsPlaying(),
		}

		libraryModel, _ := m.libraryModel.Update(statusMsg)
		m.libraryModel = libraryModel.(*LibraryModel)

		if m.currentView == LibraryView {
			positionMsg := PlaybackPositionMsg{
				Position:      m.AudioPlayer.GetPlaybackPosition(),
				TotalDuration: m.AudioPlayer.GetTotalLength(),
			}
			libraryModel, _ = m.libraryModel.Update(positionMsg)
			m.libraryModel = libraryModel.(*LibraryModel)
		}

		if err := m.AudioPlayer.GetLastError(); err != nil && err != m.lastError {
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

	case DownloaderView:
		downloaderModel, cmd := m.downloaderModel.Update(msg)
		m.downloaderModel = downloaderModel.(*DownloaderModel)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleSongSelection(msg SongSelectedMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if !msg.KeepView {
		m.currentView = PlayerView
	}

	m.currentSongIndex = m.libraryModel.FindSongIndex(msg.Song)

	libraryModel, _ := m.libraryModel.Update(msg)
	m.libraryModel = libraryModel.(*LibraryModel)

	if m.SelectedSong != nil && m.SelectedSong.Path == msg.Song.Path {
		playerModel, playerCmd := m.playerModel.Update(msg)
		m.playerModel = playerModel.(*PlayerModel)
		return m, playerCmd
	}

	m.SelectedSong = &msg.Song

	rawDominantColor := m.albumArtRenderer.ExtractDominantColor(msg.Song)
	m.dominantColor = Colors.AdjustColorForContrast(rawDominantColor)

	colorMsg := DominantColorMsg{Color: m.dominantColor}
	dModel, _ := m.downloaderModel.Update(colorMsg)
	m.downloaderModel = dModel.(*DownloaderModel)
	lModel, _ := m.libraryModel.Update(colorMsg)
	m.libraryModel = lModel.(*LibraryModel)

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

	if m.AudioPlayer.IsPlaying() {
		m.AudioPlayer.Stop()
	}

	if song.Path == "" {
		return fmt.Errorf("invalid song path")
	}

	if err := m.AudioPlayer.Load(song.Path); err != nil {
		return fmt.Errorf("failed to load song '%s': %w", song.Title, err)
	}

	if err := m.AudioPlayer.Play(); err != nil {
		return fmt.Errorf("failed to play song '%s': %w", song.Title, err)
	}

	m.AudioPlayer.ForceGC()

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
	case DownloaderView:
		return m.downloaderModel.View()
	default:
		return "Unknown view"
	}
}

func (m *Model) play() tea.Cmd {
	if m.AudioPlayer.IsPlaying() {
		return nil
	}

	err := m.AudioPlayer.Play()
	if err != nil {
		return func() tea.Msg {
			return ErrorMsg{Error: err}
		}
	}

	return func() tea.Msg {
		return PlayPauseMsg{}
	}
}

func (m *Model) pause() tea.Cmd {
	if !m.AudioPlayer.IsPlaying() {
		return nil
	}

	err := m.AudioPlayer.Pause()
	if err != nil {
		return func() tea.Msg {
			return ErrorMsg{Error: err}
		}
	}

	return func() tea.Msg {
		return PlayPauseMsg{}
	}
}

func (m *Model) playPause() tea.Cmd {
	if m.AudioPlayer.IsPlaying() {
		err := m.AudioPlayer.Pause()
		if err != nil {
			return func() tea.Msg {
				return ErrorMsg{Error: err}
			}
		}
	} else {
		err := m.AudioPlayer.Play()
		if err != nil {
			return func() tea.Msg {
				return ErrorMsg{Error: err}
			}
		}
	}

	m.playerModel.updatePlaybackStatus()

	return func() tea.Msg {
		return PlayPauseMsg{}
	}
}

func (m *Model) stop() tea.Cmd {
	if m.AudioPlayer.IsPlaying() {
		err := m.AudioPlayer.Stop()
		if err != nil {
			return func() tea.Msg {
				return ErrorMsg{Error: err}
			}
		}
	}

	m.playerModel.currentSong = nil
	m.playerModel.isPlaying = false
	m.playerModel.position = 0
	m.playerModel.totalDuration = 0
	m.playerModel.lastUpdate = time.Now()

	return func() tea.Msg {
		return StopMsg{}
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
		return SongSelectedMsg{Song: nextSong, KeepView: true}
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
		return SongSelectedMsg{Song: prevSong, KeepView: true}
	}
}

const (
	Play      = "play"
	Pause     = "pause"
	PlayPause = "playpause"
	NextTrack = "next"
	PrevTrack = "prev"
	Stop      = "stop"
)

func (m *Model) ControlPlayback(action string) error {
	if m.AudioPlayer == nil {
		return fmt.Errorf("audio player not initialized")
	}

	switch action {
	case Play:
		cmd := m.play()
		if cmd != nil {
			msg := cmd()
			_, _ = m.Update(msg)
		}
	case Pause:
		cmd := m.pause()
		if cmd != nil {
			msg := cmd()
			_, _ = m.Update(msg)
		}
	case PlayPause:
		cmd := m.playPause()
		if cmd != nil {
			msg := cmd()
			_, _ = m.Update(msg)
		}
	case Stop:
		cmd := m.stop()
		if cmd != nil {
			msg := cmd()
			_, _ = m.Update(msg)
		}
	case NextTrack:
		cmd := m.playNextTrack()
		if cmd != nil {
			msg := cmd()
			_, _ = m.Update(msg)
		}
	case PrevTrack:
		cmd := m.playPreviousTrack()
		if cmd != nil {
			msg := cmd()
			_, _ = m.Update(msg)
		}
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}
