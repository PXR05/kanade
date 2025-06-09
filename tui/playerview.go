package tui

import (
	"fmt"
	"strings"
	"time"

	"gmp/audio"
	lib "gmp/library"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PlayerModel struct {
	audioPlayer      *audio.Player
	currentSong      *lib.Song
	isPlaying        bool
	position         time.Duration
	totalDuration    time.Duration
	volume           float64
	lastVolumeChange time.Time
	showVolumeBar    bool
	errorMsg         string
	width            int
	height           int
	styles           PlayerStyles
	lastUpdate       time.Time
	albumArtRenderer *AlbumArtRenderer
	wasPlaying       bool

	lastTrackChange  time.Time
	trackChangeDelay time.Duration
}

type PlayerStyles struct {
	Title         lipgloss.Style
	Metadata      lipgloss.Style
	MetadataLabel lipgloss.Style
	ProgressBar   lipgloss.Style
	ProgressFill  lipgloss.Style
	Controls      lipgloss.Style
	Help          lipgloss.Style
	Error         lipgloss.Style
	Container     lipgloss.Style
	PlayIcon      lipgloss.Style
	PauseIcon     lipgloss.Style
}

func NewPlayerModel(audioPlayer *audio.Player) *PlayerModel {
	return &PlayerModel{
		audioPlayer:      audioPlayer,
		volume:           0.5,
		styles:           DefaultPlayerStyles(),
		lastUpdate:       time.Now(),
		albumArtRenderer: NewAlbumArtRenderer(20, 20),
		trackChangeDelay: 100 * time.Millisecond,
	}
}

func DefaultPlayerStyles() PlayerStyles {
	return PlayerStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true),
		Metadata: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Margin(0, 1),
		MetadataLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true),
		ProgressBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#3C3C3C")).
			Height(1),
		ProgressFill: lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Height(1),
		Controls: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Margin(1, 0).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Margin(1, 0),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true).
			Margin(1, 0),
		Container: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")),
		PlayIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true),
		PauseIcon: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB86C")).
			Bold(true),
	}
}

func (m *PlayerModel) Init() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

func (m *PlayerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.albumArtRenderer = NewResponsiveAlbumArtRenderer(m.width, m.height)
		return m, nil

	case SongSelectedMsg:
		m.currentSong = &msg.Song
		m.errorMsg = ""
		if m.audioPlayer != nil {
			m.audioPlayer.SetVolume(m.volume)
		}
		m.updatePlaybackStatus()

		return m, tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		})

	case PlaybackStatusMsg:
		if msg.Error != nil {
			m.errorMsg = msg.Error.Error()
		} else {
			m.errorMsg = ""
		}
		m.isPlaying = msg.IsPlaying
		return m, nil

	case TickMsg:

		if m.audioPlayer != nil && m.currentSong != nil {
			m.updatePlaybackStatus()

			if m.wasPlaying && !m.isPlaying && m.position >= m.totalDuration-time.Millisecond*100 {
				m.wasPlaying = false
				return m, func() tea.Msg {
					return SongFinishedMsg{}
				}
			}
			m.wasPlaying = m.isPlaying
		}

		if m.showVolumeBar && time.Since(m.lastVolumeChange) > 2*time.Second {
			m.showVolumeBar = false
		}

		return m, tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		})

	case tea.KeyMsg:
		if m.audioPlayer == nil {
			return m, nil
		}

		switch msg.String() {
		case "tab":
			if m.currentSong != nil {
				return m, func() tea.Msg {
					return SwitchViewMsg{View: LibraryView}
				}
			}

		case " ", "p":

			if m.isPlaying {
				err := m.audioPlayer.Pause()
				if err != nil {
					m.errorMsg = err.Error()
				}
			} else {
				err := m.audioPlayer.Play()
				if err != nil {
					m.errorMsg = err.Error()
				}
			}
			m.updatePlaybackStatus()

		case "s":

			err := m.audioPlayer.Stop()
			if err != nil {
				m.errorMsg = err.Error()
			}
			m.updatePlaybackStatus()

		case "left", "h":

			currentPos := m.audioPlayer.GetPlaybackPosition()
			newPos := max(currentPos-10*time.Second, 0)
			err := m.audioPlayer.Seek(newPos)
			if err != nil {
				m.errorMsg = err.Error()
			} else {
				m.updatePlaybackStatus()
			}

		case "right", "l":

			currentPos := m.audioPlayer.GetPlaybackPosition()
			newPos := min(currentPos+10*time.Second, m.totalDuration)
			err := m.audioPlayer.Seek(newPos)
			if err != nil {
				m.errorMsg = err.Error()
			} else {
				m.updatePlaybackStatus()
			}

		case "shift+left", "shift+h", "H":

			if time.Since(m.lastTrackChange) < m.trackChangeDelay {
				return m, nil
			}
			m.lastTrackChange = time.Now()

			return m, func() tea.Msg {
				return PrevTrackMsg{}
			}

		case "shift+right", "shift+l", "L":

			if time.Since(m.lastTrackChange) < m.trackChangeDelay {
				return m, nil
			}
			m.lastTrackChange = time.Now()

			return m, func() tea.Msg {
				return NextTrackMsg{}
			}

		case "0":

			err := m.audioPlayer.Seek(0)
			if err != nil {
				m.errorMsg = err.Error()
			} else {
				m.updatePlaybackStatus()
			}

		case "d":

			if m.albumArtRenderer != nil {
				m.errorMsg = "Terminal Info:\n" + m.albumArtRenderer.GetTerminalInfo()
			}

		case "up", "=":
			newVolume := min(m.volume+0.1, 1.0)
			err := m.audioPlayer.SetVolume(newVolume)
			if err != nil {
				m.errorMsg = err.Error()
			} else {
				m.volume = newVolume
				m.lastVolumeChange = time.Now()
				m.showVolumeBar = true
				m.errorMsg = ""
			}

		case "down", "-":
			newVolume := max(m.volume-0.1, 0.0)
			err := m.audioPlayer.SetVolume(newVolume)
			if err != nil {
				m.errorMsg = err.Error()
			} else {
				m.volume = newVolume
				m.lastVolumeChange = time.Now()
				m.showVolumeBar = true
				m.errorMsg = ""
			}

		case "m":
			if m.volume > 0 {
				err := m.audioPlayer.SetVolume(0.0)
				if err != nil {
					m.errorMsg = err.Error()
				} else {
					m.volume = 0.0
					m.lastVolumeChange = time.Now()
					m.showVolumeBar = true
					m.errorMsg = ""
				}
			} else {

				err := m.audioPlayer.SetVolume(0.7)
				if err != nil {
					m.errorMsg = err.Error()
				} else {
					m.volume = 0.7
					m.lastVolumeChange = time.Now()
					m.showVolumeBar = true
					m.errorMsg = ""
				}
			}
		}
	}

	return m, nil
}

func (m *PlayerModel) updatePlaybackStatus() {
	if m.audioPlayer == nil {
		return
	}

	m.isPlaying = m.audioPlayer.IsPlaying()
	m.position = m.audioPlayer.GetPlaybackPosition()
	m.totalDuration = m.audioPlayer.GetTotalLength()
	m.lastUpdate = time.Now()
}

func (m *PlayerModel) View() string {
	var content strings.Builder

	if m.currentSong == nil {

		centerStyle := lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center)

		content.WriteString(centerStyle.Render("No song selected"))
		content.WriteString("\n\n")

		helpStyle := lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#626262"))
		content.WriteString(helpStyle.Render("Press Esc, 'q', or Tab to go back to library"))
		content.WriteString("\n")
		content.WriteString(helpStyle.Render("Controls: Space/p:play/pause • ←/→/h/l:seek • Shift+←/→/h/l:prev/next track • ↑/↓/+/-:volume • m:mute"))

		return content.String()
	}

	rawDominantColor := m.albumArtRenderer.ExtractDominantColor(*m.currentSong)
	dominantColor := Colors.AdjustColorForContrast(rawDominantColor)

	availableHeight := m.height - 10
	contentHeight := 20
	topPadding := max((availableHeight-contentHeight)/2, 2)

	for range topPadding {
		content.WriteString("\n")
	}

	albumArt := m.albumArtRenderer.RenderAlbumArt(*m.currentSong)
	albumArtLines := strings.SplitSeq(albumArt, "\n")
	for line := range albumArtLines {
		if line != "" {
			centerStyle := lipgloss.NewStyle().
				Width(m.width).
				Align(lipgloss.Center)
			content.WriteString(centerStyle.Render(line))
		}
		content.WriteString("\n")
	}

	titleStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("#FAFAFA")).
		Bold(true)
	content.WriteString(titleStyle.Render(m.currentSong.Title))
	content.WriteString("\n")

	artistStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color(dominantColor))
	content.WriteString(artistStyle.Render(m.currentSong.Artist))
	content.WriteString("\n\n")

	if m.totalDuration > 0 {
		progressWidth := 40

		progress := float64(m.position) / float64(m.totalDuration)
		if progress > 1.0 {
			progress = 1.0
		}

		progressBar := m.generateStableProgressBar(progressWidth, progress, dominantColor)

		progressStyle := lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center)
		content.WriteString(progressStyle.Render(progressBar))
		content.WriteString("\n\n")
	}

	if m.showVolumeBar {
		volumeWidth := 20
		volumeProgress := m.volume
		volumeBar := m.generateStableProgressBar(volumeWidth, volumeProgress, dominantColor)

		volumeIcon := ""
		if m.volume == 0 {
			volumeIcon = ""
		} else if m.volume < 0.5 {
			volumeIcon = ""
		}

		volumeText := fmt.Sprintf("%s %s %d%%", volumeIcon, volumeBar, int(m.volume*100))
		volumeStyle := lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color(dominantColor))
		content.WriteString(volumeStyle.Render(volumeText))
		content.WriteString("\n\n")
	}

	if m.errorMsg != "" {
		content.WriteString("\n")
		errorStyle := lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)
		content.WriteString(errorStyle.Render("Error: " + m.errorMsg))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *PlayerModel) generateStableProgressBar(width int, progress float64, dominantColor string) string {

	blocks := []string{"░", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}

	exactPos := progress * float64(width)
	filledColor := dominantColor
	emptyColor := Colors.DarkenColor(dominantColor, 0.7)

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(filledColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor))

	var finalBar strings.Builder

	for i := range width {
		charProgress := exactPos - float64(i)

		var char string
		var style lipgloss.Style

		if charProgress <= 0 {

			char = "░"
			style = emptyStyle
		} else if charProgress >= 1.0 {

			char = "█"
			style = filledStyle
		} else {

			blockIndex := int(charProgress*8) + 1
			if blockIndex > 8 {
				blockIndex = 8
			}
			if blockIndex < 1 {
				blockIndex = 1
			}
			char = blocks[blockIndex]
			style = filledStyle
		}

		finalBar.WriteString(style.Render(char))
	}

	return finalBar.String()
}
