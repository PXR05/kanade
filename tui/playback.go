package tui

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	lib "kanade/library"

	tea "github.com/charmbracelet/bubbletea"
)

type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
)

func (r RepeatMode) String() string {
	switch r {
	case RepeatAll:
		return "all"
	case RepeatOne:
		return "one"
	default:
		return "off"
	}
}

func (r RepeatMode) Label() string {
	switch r {
	case RepeatAll:
		return "Repeat all"
	case RepeatOne:
		return "Repeat one"
	default:
		return "Repeat off"
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

func (m *Model) play() tea.Cmd {
	if m.AudioPlayer.IsPlaying() || m.SelectedSong == nil {
		return nil
	}
	if err := m.AudioPlayer.Play(); err != nil {
		return func() tea.Msg { return ErrorMsg{Error: err} }
	}
	m.playerModel.updatePlaybackStatus()
	return nil
}

func (m *Model) pause() tea.Cmd {
	if !m.AudioPlayer.IsPlaying() {
		return nil
	}
	if err := m.AudioPlayer.Pause(); err != nil {
		return func() tea.Msg { return ErrorMsg{Error: err} }
	}
	m.playerModel.updatePlaybackStatus()
	return nil
}

func (m *Model) togglePlay() tea.Cmd {
	if m.AudioPlayer.IsPlaying() {
		return m.pause()
	}
	return m.play()
}

func (m *Model) stopPlayback() tea.Cmd {
	m.finishHandled = false
	if err := m.AudioPlayer.Stop(); err != nil && m.SelectedSong != nil {
		return func() tea.Msg { return ErrorMsg{Error: err} }
	}
	m.playerModel.reset()
	return nil
}

func (m *Model) nextTrack(manual bool) tea.Cmd {
	songs := m.libraryModel.GetOrderedSongs()
	n := len(songs)
	if n == 0 || m.currentSongIndex < 0 {
		return nil
	}

	idx := m.pickNextIndex(n, manual)
	if idx < 0 {
		return m.stopPlayback()
	}
	return func() tea.Msg { return SongSelectedMsg{Song: songs[idx], KeepView: true} }
}

func (m *Model) prevTrack() tea.Cmd {
	songs := m.libraryModel.GetOrderedSongs()
	n := len(songs)
	if n == 0 || m.currentSongIndex < 0 {
		return nil
	}

	idx := m.currentSongIndex - 1
	if idx < 0 {
		idx = n - 1
	}
	return func() tea.Msg { return SongSelectedMsg{Song: songs[idx], KeepView: true} }
}

func (m *Model) pickNextIndex(n int, manual bool) int {
	if m.shuffle && n > 1 {
		idx := rand.IntN(n - 1)
		if idx >= m.currentSongIndex {
			idx++
		}
		return idx
	}

	idx := m.currentSongIndex + 1
	if idx >= n {
		if !manual && m.repeatMode != RepeatAll {
			return -1
		}
		idx = 0
	}
	return idx
}

func (m *Model) handleSongFinished() tea.Cmd {
	if m.repeatMode == RepeatOne && m.SelectedSong != nil {
		song := *m.SelectedSong
		return func() tea.Msg { return SongSelectedMsg{Song: song, KeepView: true} }
	}
	return m.nextTrack(false)
}

func (m *Model) selectSong(song lib.Song, keepView bool) {
	if !keepView {
		m.currentView = PlayerView
	}

	m.finishHandled = false
	m.currentSongIndex = m.libraryModel.FindSongIndex(song)
	m.SelectedSong = &song

	m.libraryModel.currentSong = &song
	m.playerModel.setSong(&song)

	if m.coloredSongPath != song.Path {
		raw := m.albumArtRenderer.ExtractDominantColor(song)
		m.dominantColor = Colors.AdjustColorForContrast(raw)
		m.coloredSongPath = song.Path
	}
	m.libraryModel.dominantColor = m.dominantColor

	if err := m.loadAndPlaySong(song); err != nil {
		m.lastError = err
		m.statusText = err.Error()
		m.statusIsError = true
		m.statusExpiry = time.Now().Add(StatusTimeout)
		return
	}
	m.playerModel.updatePlaybackStatus()
}

func (m *Model) loadAndPlaySong(song lib.Song) error {
	if song.Path == "" {
		return fmt.Errorf("invalid song path")
	}
	if m.AudioPlayer.IsPlaying() {
		m.AudioPlayer.Stop()
	}
	if err := m.AudioPlayer.Load(song.Path); err != nil {
		return fmt.Errorf("failed to load '%s': %w", song.Title, err)
	}
	if err := m.AudioPlayer.Play(); err != nil {
		return fmt.Errorf("failed to play '%s': %w", song.Title, err)
	}
	m.AudioPlayer.ForceGC()
	return nil
}

func (m *Model) cycleRepeatMode() tea.Cmd {
	m.repeatMode = (m.repeatMode + 1) % 3
	m.syncPlaybackModes()
	return m.showToast(fmt.Sprintf("%s  (%s)", modeIcon(m.repeatMode, m.shuffle), m.repeatMode), false)
}

func (m *Model) toggleShuffle() tea.Cmd {
	m.shuffle = !m.shuffle
	m.syncPlaybackModes()
	state := "off"
	if m.shuffle {
		state = "on"
	}
	return m.showToast(fmt.Sprintf("Shuffle %s", state), false)
}

func (m *Model) syncPlaybackModes() {
	m.libraryModel.repeatMode = m.repeatMode
	m.libraryModel.shuffle = m.shuffle
	m.playerModel.repeatMode = m.repeatMode
	m.playerModel.shuffle = m.shuffle
}

func (m *Model) changeVolume(delta float64) tea.Cmd {
	v := ClampFloat64(m.volume+delta, 0.0, 1.0)
	if v == m.volume {
		return nil
	}
	return m.setVolume(v)
}

func (m *Model) setVolume(v float64) tea.Cmd {
	if err := m.AudioPlayer.SetVolume(v); err != nil {
		return func() tea.Msg { return ErrorMsg{Error: err} }
	}
	m.volume = v
	if v > 0 {
		m.muted = false
	}
	m.playerModel.showVolume(v)
	return nil
}

func (m *Model) toggleMute() tea.Cmd {
	if m.muted || m.volume <= 0 {
		m.setVolume(m.preMuteVol)
		return m.showToast("Unmuted", false)
	}
	m.preMuteVol = m.volume
	if err := m.AudioPlayer.SetVolume(0); err != nil {
		return func() tea.Msg { return ErrorMsg{Error: err} }
	}
	m.muted = true
	m.playerModel.showVolume(0)
	return m.showToast("Muted", false)
}

func modeIcon(repeat RepeatMode, shuffle bool) string {
	parts := []string{}
	switch repeat {
	case RepeatAll:
		parts = append(parts, "⟳ all")
	case RepeatOne:
		parts = append(parts, "⟳¹ one")
	}
	if shuffle {
		parts = append(parts, "⇄ shuffle")
	}
	return strings.Join(parts, "  ")
}

func (m *Model) rescanLibrary() tea.Cmd {
	songs, warnings, err := m.library.Reload(m.libraryDir)
	if err != nil {
		return func() tea.Msg { return ErrorMsg{Error: fmt.Errorf("rescan failed: %w", err)} }
	}
	m.libraryModel.SetSongs(songs)
	if m.SelectedSong != nil {
		m.currentSongIndex = m.libraryModel.FindSongIndex(*m.SelectedSong)
	}

	msg := fmt.Sprintf("Rescanned: %d songs", len(songs))
	if len(warnings) > 0 {
		msg += fmt.Sprintf(" (%d skipped)", len(warnings))
	}
	return m.showToast(msg, false)
}

func (m *Model) ControlPlayback(action string) error {
	if m.AudioPlayer == nil {
		return fmt.Errorf("audio player not initialized")
	}

	var msg tea.Msg
	switch action {
	case Play:
		msg = PlayMsg{}
	case Pause:
		msg = PauseMsg{}
	case PlayPause:
		msg = TogglePlayMsg{}
	case Stop:
		msg = StopRequestMsg{}
	case NextTrack:
		msg = NextTrackMsg{}
	case PrevTrack:
		msg = PrevTrackMsg{}
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	if m.sendFn != nil {
		m.sendFn(msg)
		return nil
	}

	switch msg.(type) {
	case PlayMsg:
		m.play()
	case PauseMsg:
		m.pause()
	case TogglePlayMsg:
		m.togglePlay()
	case StopRequestMsg:
		m.stopPlayback()
	case NextTrackMsg:
		if cmd := m.nextTrack(true); cmd != nil {
			selectSongFromCmd(m, cmd)
		}
	case PrevTrackMsg:
		if cmd := m.prevTrack(); cmd != nil {
			selectSongFromCmd(m, cmd)
		}
	}
	return nil
}

func selectSongFromCmd(m *Model, cmd tea.Cmd) {
	if msg, ok := cmd().(SongSelectedMsg); ok {
		m.selectSong(msg.Song, msg.KeepView)
	}
}
