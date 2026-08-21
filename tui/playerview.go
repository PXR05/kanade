package tui

import (
	"strings"
	"time"

	"kanade/audio"
	lib "kanade/library"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PlayerModel struct {
	audioPlayer   *audio.Player
	currentSong   *lib.Song
	isPlaying     bool
	position      time.Duration
	totalDuration time.Duration
	volumeLevel   float64
	showVolumeBar bool
	lastVolumeChg time.Time
	width         int
	height        int
	albumArtRend  *AlbumArtRenderer

	cachedDominantColor string
	cachedAlbumArt      string
	cachedSongPath      string

	barY   int
	barX   int
	barW   int
	hasBar bool

	repeatMode RepeatMode
	shuffle    bool

	statusText    string
	statusIsError bool
	statusExpiry  time.Time
}

func NewPlayerModel(audioPlayer *audio.Player) *PlayerModel {
	return &PlayerModel{
		audioPlayer:  audioPlayer,
		volumeLevel:  audioPlayer.GetVolume(),
		albumArtRend: NewAlbumArtRenderer(AlbumArtMinMax, AlbumArtMinMax),
	}
}

func (m *PlayerModel) setSong(song *lib.Song) {
	m.currentSong = song
	m.updatePlaybackStatus()
	if m.audioPlayer != nil {
		m.volumeLevel = m.audioPlayer.GetVolume()
	}
}

func (m *PlayerModel) reset() {
	m.currentSong = nil
	m.isPlaying = false
	m.position = 0
	m.totalDuration = 0
	m.hasBar = false
}

func (m *PlayerModel) showVolume(level float64) {
	m.volumeLevel = level
	m.showVolumeBar = true
	m.lastVolumeChg = time.Now()
}

func (m *PlayerModel) updatePlaybackStatus() {
	if m.audioPlayer == nil {
		return
	}
	m.isPlaying = m.audioPlayer.IsPlaying()
	m.position = m.audioPlayer.GetPlaybackPosition()
	m.totalDuration = m.audioPlayer.GetTotalLength()
}

func (m *PlayerModel) Init() tea.Cmd {
	return nil
}

func (m *PlayerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.albumArtRend = NewResponsiveAlbumArtRenderer(m.width, m.height)
		return m, nil

	case SongSelectedMsg:
		m.setSong(&msg.Song)
		return m, nil

	case PlaybackStatusMsg:
		m.isPlaying = msg.IsPlaying
		return m, nil

	case PlaybackPositionMsg:
		m.position = msg.Position
		m.totalDuration = msg.TotalDuration
		return m, nil

	case StatusMsg:
		m.statusText = msg.Text
		m.statusIsError = msg.IsError
		m.statusExpiry = time.Now().Add(StatusTimeout)
		return m, nil

	case tea.KeyMsg:
		if m.audioPlayer == nil || m.currentSong == nil {
			return m, nil
		}

		switch msg.String() {
		case " ":
			if err := m.togglePlay(); err != nil {
				return m, func() tea.Msg { return ErrorMsg{Error: err} }
			}
			m.updatePlaybackStatus()

		case "s":
			if err := m.audioPlayer.Stop(); err != nil {
				return m, func() tea.Msg { return ErrorMsg{Error: err} }
			}
			m.updatePlaybackStatus()

		case "left", "h":
			newPos := max(m.audioPlayer.GetPlaybackPosition()-SeekInterval, 0)
			if err := m.audioPlayer.Seek(newPos); err != nil {
				return m, func() tea.Msg { return ErrorMsg{Error: err} }
			}
			m.updatePlaybackStatus()

		case "right", "l":
			newPos := min(m.audioPlayer.GetPlaybackPosition()+SeekInterval, m.totalDuration)
			if err := m.audioPlayer.Seek(newPos); err != nil {
				return m, func() tea.Msg { return ErrorMsg{Error: err} }
			}
			m.updatePlaybackStatus()

		case "shift+left", "shift+h":
			return m, func() tea.Msg { return PrevTrackMsg{} }

		case "shift+right", "shift+l":
			return m, func() tea.Msg { return NextTrackMsg{} }

		case "0":
			if err := m.audioPlayer.Seek(0); err != nil {
				return m, func() tea.Msg { return ErrorMsg{Error: err} }
			}
			m.updatePlaybackStatus()
		}
	}

	return m, nil
}

func (m *PlayerModel) togglePlay() error {
	if m.isPlaying {
		return m.audioPlayer.Pause()
	}
	return m.audioPlayer.Play()
}

func (m *PlayerModel) HandleClick(msg tea.MouseMsg) tea.Cmd {
	if !m.hasBar || m.totalDuration <= 0 {
		return nil
	}
	if msg.Y < m.barY-1 || msg.Y > m.barY+1 {
		return nil
	}
	if msg.X < m.barX || msg.X >= m.barX+m.barW {
		return nil
	}

	ratio := ClampFloat64(float64(msg.X-m.barX)/float64(m.barW), 0.0, 1.0)
	target := time.Duration(ratio * float64(m.totalDuration))
	if err := m.audioPlayer.Seek(target); err != nil {
		return func() tea.Msg { return ErrorMsg{Error: err} }
	}
	m.updatePlaybackStatus()
	return nil
}

func (m *PlayerModel) View() string {
	var content strings.Builder

	if m.currentSong == nil {
		topPadding := max(m.height/2-2, 0)
		for range topPadding {
			content.WriteString("\n")
		}
		centerStyle := lipgloss.NewStyle().
			Width(max(m.width, ContentMinWidth)).
			Align(lipgloss.Center)
		content.WriteString(centerStyle.Render("No song selected"))
		content.WriteString("\n\n")
		content.WriteString(centerStyle.Render("Pick a song in the library (tab)"))
		content.WriteString("\n")
		return content.String()
	}

	dominantColor := m.dominantColorFor()

	availableHeight := m.height - DefaultPadding*5
	contentHeight := DefaultPadding * 10
	topPadding := max((availableHeight-contentHeight)/DefaultPadding, DefaultPadding)

	lineCount := 0
	for range topPadding {
		content.WriteString("\n")
		lineCount++
	}

	artLines := strings.SplitSeq(m.albumArtFor(), "\n")
	for line := range artLines {
		if line != "" {
			centerStyle := lipgloss.NewStyle().
				Width(max(m.width, ContentMinWidth)).
				Align(lipgloss.Center)
			content.WriteString(centerStyle.Render(line))
		}
		content.WriteString("\n")
		lineCount++
	}

	titleStyle := lipgloss.NewStyle().
		Width(max(m.width, ContentMinWidth)).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true)
	content.WriteString(titleStyle.Render(TruncateString(m.currentSong.Title, max(m.width, ContentMinWidth))))
	content.WriteString("\n")
	lineCount++

	artistText := m.currentSong.Artist
	if artistText == "" {
		artistText = "Unknown Artist"
	}
	artistStyle := lipgloss.NewStyle().
		Width(max(m.width, ContentMinWidth)).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color(dominantColor))
	content.WriteString(artistStyle.Render(artistText))
	content.WriteString("\n\n")
	lineCount += 2

	if m.showVolumeBar && time.Since(m.lastVolumeChg) > VolumeBarTimeout {
		m.showVolumeBar = false
	}

	if m.totalDuration > 0 {
		progress := ClampFloat64(float64(m.position)/float64(m.totalDuration), 0.0, 1.0)

		leftTime := FormatDuration(m.position)
		rightTime := FormatDuration(m.totalDuration)
		barWidth := ProgressBarWidth
		xStart := max((m.width-lipgloss.Width(leftTime)-2-barWidth-2-lipgloss.Width(rightTime))/2, 0)

		bar := RenderProgressBar(barWidth, progress, dominantColor)
		row := leftTime + "  " + bar + "  " + rightTime

		content.WriteString(strings.Repeat(" ", xStart) + row)
		content.WriteString("\n")

		m.barY = lineCount
		m.barX = xStart + lipgloss.Width(leftTime) + 2
		m.barW = barWidth
		m.hasBar = true
		lineCount++

		if m.showVolumeBar {
			volumeProgress := ClampFloat64(m.volumeLevel, 0.0, 1.0)
			volumeBar := RenderProgressBar(VolumeBarWidth, volumeProgress, dominantColor)
			volumeStyle := lipgloss.NewStyle().
				Width(max(m.width, ContentMinWidth)).
				Align(lipgloss.Center).
				Foreground(lipgloss.Color(dominantColor))
			content.WriteString(volumeStyle.Render(volumeBar))
			content.WriteString("\n")
			lineCount++
		}
		content.WriteString("\n")
		lineCount++

		if indicators := modeIcon(m.repeatMode, m.shuffle); indicators != "" {
			modeStyle := lipgloss.NewStyle().
				Width(max(m.width, ContentMinWidth)).
				Align(lipgloss.Center).
				Foreground(lipgloss.Color(dominantColor))
			content.WriteString(modeStyle.Render(indicators))
			content.WriteString("\n")
			lineCount++
		}
	} else {
		m.hasBar = false
	}

	if m.statusActive() {
		statusStyle := lipgloss.NewStyle().
			Width(max(m.width, ContentMinWidth)).
			Align(lipgloss.Center)
		color := DefaultWarningColor
		if m.statusIsError {
			color = DefaultErrorColor
		}
		statusStyle = statusStyle.Foreground(lipgloss.Color(color)).Bold(m.statusIsError)
		content.WriteString(statusStyle.Render(TruncateString(m.statusText, max(m.width-4, ContentMinWidth))))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *PlayerModel) statusActive() bool {
	return m.statusText != "" && time.Now().Before(m.statusExpiry)
}

func (m *PlayerModel) dominantColorFor() string {
	if m.currentSong == nil {
		return DefaultAccentColor
	}
	if m.currentSong.Path != m.cachedSongPath {
		raw := m.albumArtRend.ExtractDominantColor(*m.currentSong)
		m.cachedDominantColor = Colors.AdjustColorForContrast(raw)
		m.cachedAlbumArt = m.albumArtRend.RenderAlbumArt(*m.currentSong)
		m.cachedSongPath = m.currentSong.Path
	}
	return m.cachedDominantColor
}

func (m *PlayerModel) albumArtFor() string {
	m.dominantColorFor()
	return m.cachedAlbumArt
}
