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

// PlayerModel represents the player view model
type PlayerModel struct {
	audioPlayer      *audio.Player
	currentSong      *lib.Song
	isPlaying        bool
	position         time.Duration
	totalDuration    time.Duration
	errorMsg         string
	width            int
	height           int
	styles           PlayerStyles
	lastUpdate       time.Time
	albumArtRenderer *AlbumArtRenderer
}

// PlayerStyles contains styling for the player view
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

// NewPlayerModel creates a new player view model
func NewPlayerModel(audioPlayer *audio.Player) *PlayerModel {
	return &PlayerModel{
		audioPlayer:      audioPlayer,
		styles:           DefaultPlayerStyles(),
		lastUpdate:       time.Now(),
		albumArtRenderer: NewAlbumArtRenderer(20, 10), // Default size
	}
}

// DefaultPlayerStyles returns default styling for the player view
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

// Init initializes the player model
func (m *PlayerModel) Init() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

// TickMsg is sent periodically to update the playback position
type TickMsg struct {
	Time time.Time
}

// Update handles messages for the player view
func (m *PlayerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update album art renderer size based on available space
		artWidth := 25
		artHeight := 12
		if m.width < 80 {
			artWidth = 20
			artHeight = 10
		}
		m.albumArtRenderer = NewAlbumArtRenderer(artWidth, artHeight)
		return m, nil

	case SongSelectedMsg:
		m.currentSong = &msg.Song
		m.errorMsg = ""
		m.updatePlaybackStatus()
		return m, nil

	case PlaybackStatusMsg:
		if msg.Error != nil {
			m.errorMsg = msg.Error.Error()
		} else {
			m.errorMsg = ""
		}
		m.isPlaying = msg.IsPlaying
		return m, nil

	case TickMsg:
		// Update playback position periodically
		if m.audioPlayer != nil {
			m.updatePlaybackStatus()
		}
		// Continue ticking for real-time updates
		return m, tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		})

	case tea.KeyMsg:
		if m.audioPlayer == nil {
			return m, nil
		}

		switch msg.String() {
		case " ", "p":
			// Toggle play/pause
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
			// Stop
			err := m.audioPlayer.Stop()
			if err != nil {
				m.errorMsg = err.Error()
			}
			m.updatePlaybackStatus()

		case "left", "h":
			// Seek backward 10 seconds
			currentPos := m.audioPlayer.GetPlaybackPosition()
			newPos := currentPos - 10*time.Second
			if newPos < 0 {
				newPos = 0
			}
			err := m.audioPlayer.Seek(newPos)
			if err != nil {
				m.errorMsg = err.Error()
			}

		case "right", "l":
			// Seek forward 10 seconds
			currentPos := m.audioPlayer.GetPlaybackPosition()
			newPos := currentPos + 10*time.Second
			if newPos > m.totalDuration {
				newPos = m.totalDuration
			}
			err := m.audioPlayer.Seek(newPos)
			if err != nil {
				m.errorMsg = err.Error()
			}

		case "0":
			// Seek to beginning
			err := m.audioPlayer.Seek(0)
			if err != nil {
				m.errorMsg = err.Error()
			}

		case "d":
			// Debug: Show terminal capabilities
			if m.albumArtRenderer != nil {
				m.errorMsg = "Terminal Info:\n" + m.albumArtRenderer.GetTerminalInfo()
			}
		}
	}

	return m, nil
}

// updatePlaybackStatus updates the current playback status
func (m *PlayerModel) updatePlaybackStatus() {
	if m.audioPlayer == nil {
		return
	}

	m.isPlaying = m.audioPlayer.IsPlaying()
	m.position = m.audioPlayer.GetPlaybackPosition()
	m.totalDuration = m.audioPlayer.GetTotalLength()
}

// View renders the player view
func (m *PlayerModel) View() string {
	var content strings.Builder

	// Title
	content.WriteString(m.styles.Title.Render("🎵 Now Playing"))
	content.WriteString("\n\n")

	if m.currentSong == nil {
		content.WriteString(m.styles.Metadata.Render("No song selected"))
		content.WriteString("\n\n")
		content.WriteString(m.styles.Help.Render("Press Esc or 'q' to go back to library"))
		return m.styles.Container.Render(content.String())
	}

	// Create two-column layout: album art on left, metadata on right
	var leftColumn, rightColumn strings.Builder

	// Left column: Album art
	albumArt := m.albumArtRenderer.RenderAlbumArt(*m.currentSong)
	leftColumn.WriteString(albumArt)

	// Right column: Song metadata
	rightColumn.WriteString(m.styles.MetadataLabel.Render("Title: "))
	rightColumn.WriteString(m.styles.Metadata.Render(m.currentSong.Title))
	rightColumn.WriteString("\n")

	rightColumn.WriteString(m.styles.MetadataLabel.Render("Artist: "))
	rightColumn.WriteString(m.styles.Metadata.Render(m.currentSong.Artist))
	rightColumn.WriteString("\n")

	if m.currentSong.Album != "" {
		rightColumn.WriteString(m.styles.MetadataLabel.Render("Album: "))
		rightColumn.WriteString(m.styles.Metadata.Render(m.currentSong.Album))
		rightColumn.WriteString("\n")
	}

	if m.currentSong.Genre != "" {
		rightColumn.WriteString(m.styles.MetadataLabel.Render("Genre: "))
		rightColumn.WriteString(m.styles.Metadata.Render(m.currentSong.Genre))
		rightColumn.WriteString("\n")
	}

	rightColumn.WriteString("\n")

	// Playback status
	statusIcon := "⏸️"
	statusText := "Paused"
	if m.isPlaying {
		statusIcon = "▶️"
		statusText = "Playing"
	}

	rightColumn.WriteString(m.styles.Controls.Render(fmt.Sprintf("%s %s", statusIcon, statusText)))
	rightColumn.WriteString("\n\n")

	// Combine columns side by side if there's enough width
	if m.width >= 80 {
		leftStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Center)
		rightStyle := lipgloss.NewStyle().Width(m.width - 40).Align(lipgloss.Left)

		columns := lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftStyle.Render(leftColumn.String()),
			rightStyle.Render(rightColumn.String()),
		)
		content.WriteString(columns)
	} else {
		// Stack vertically for narrow screens
		content.WriteString(leftColumn.String())
		content.WriteString("\n")
		content.WriteString(rightColumn.String())
	}

	content.WriteString("\n")

	// Progress bar and time
	if m.totalDuration > 0 {
		progressWidth := m.width - 20
		if progressWidth < 20 {
			progressWidth = 20
		}
		if progressWidth > 60 {
			progressWidth = 60
		}

		fillWidth := int(float64(progressWidth) * float64(m.position) / float64(m.totalDuration))
		if fillWidth > progressWidth {
			fillWidth = progressWidth
		}

		// Progress bar
		fill := strings.Repeat("█", fillWidth)
		empty := strings.Repeat("░", progressWidth-fillWidth)
		progressBar := m.styles.ProgressFill.Render(fill) + m.styles.ProgressBar.Render(empty)

		content.WriteString(progressBar)
		content.WriteString("\n")

		// Time display
		positionStr := formatDuration(m.position)
		totalStr := formatDuration(m.totalDuration)
		percentage := float64(m.position) / float64(m.totalDuration) * 100

		timeInfo := fmt.Sprintf("%s / %s (%.1f%%)", positionStr, totalStr, percentage)
		content.WriteString(m.styles.Metadata.Render(timeInfo))
		content.WriteString("\n\n")
	}

	// Error message
	if m.errorMsg != "" {
		content.WriteString(m.styles.Error.Render("Error: " + m.errorMsg))
		content.WriteString("\n\n")
	}

	// Help text
	content.WriteString(m.styles.Help.Render("Controls: Space/p=Play/Pause • s=Stop • ←/→=Seek±10s • 0=Beginning • d=Debug"))
	content.WriteString("\n")
	content.WriteString(m.styles.Help.Render("Navigation: Esc/q=Back to Library"))

	return m.styles.Container.Render(content.String())
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
