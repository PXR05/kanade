package tui

import (
	"fmt"
	"strings"

	lib "gmp/library"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LibraryModel struct {
	songs            []lib.Song
	cursor           int
	width            int
	height           int
	styles           LibraryStyles
	currentSong      *lib.Song
	isPlaying        bool
	albumArtRenderer *AlbumArtRenderer
}

type LibraryStyles struct {
	Title     lipgloss.Style
	Header    lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style
	Help      lipgloss.Style
	Container lipgloss.Style
}

func NewLibraryModel(songs []lib.Song) *LibraryModel {
	return &LibraryModel{
		songs:            songs,
		cursor:           0,
		styles:           DefaultLibraryStyles(),
		albumArtRenderer: NewAlbumArtRenderer(20, 20),
	}
}

func DefaultLibraryStyles() LibraryStyles {
	return LibraryStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Bold(true).
			Margin(1, 0),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Margin(1, 0),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#333333")).
			Bold(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Margin(1, 0),
		Container: lipgloss.NewStyle().
			Padding(1, 2),
	}
}

func (m *LibraryModel) GetColoredStyles(dominantColor string) LibraryStyles {
	adjustedColor := Colors.AdjustColorForContrast(dominantColor)
	backgroundAdjustedColor := Colors.DarkenColor(adjustedColor, 0.4)

	return LibraryStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Bold(true).
			Margin(1, 0),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)).
			Margin(1, 0),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color(backgroundAdjustedColor)).
			Bold(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)).
			Margin(1, 0),
		Container: lipgloss.NewStyle().
			Padding(1, 2),
	}
}

func (m *LibraryModel) Init() tea.Cmd {
	return nil
}

func (m *LibraryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.albumArtRenderer = NewResponsiveAlbumArtRenderer(m.width, m.height)
		return m, nil

	case SongSelectedMsg:

		m.currentSong = &msg.Song
		return m, nil

	case PlaybackStatusMsg:

		m.isPlaying = msg.IsPlaying
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.songs)-1 {
				m.cursor++
			}
		case "enter", " ":
			if len(m.songs) > 0 && m.cursor < len(m.songs) {

				return m, func() tea.Msg {
					return SongSelectedMsg{Song: m.songs[m.cursor]}
				}
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.songs) - 1
		}
	}

	return m, nil
}

func (m *LibraryModel) SetCurrentSong(song *lib.Song) {
	m.currentSong = song
}

func (m *LibraryModel) SetPlaybackStatus(isPlaying bool) {
	m.isPlaying = isPlaying
}

func (m *LibraryModel) View() string {

	var currentStyles LibraryStyles
	if m.currentSong != nil && m.albumArtRenderer != nil {

		dominantColor := m.albumArtRenderer.ExtractDominantColor(*m.currentSong)
		currentStyles = m.GetColoredStyles(dominantColor)
	} else {

		currentStyles = m.styles
	}

	if len(m.songs) == 0 {
		var content strings.Builder
		content.WriteString(currentStyles.Title.Render("Music Library"))
		content.WriteString("\n\n")
		content.WriteString(currentStyles.Normal.Render("No songs found"))
		content.WriteString("\n")
		content.WriteString(currentStyles.Help.Render("Add .mp3 or .wav files to the assets folder"))
		content.WriteString("\n\n")
		content.WriteString(currentStyles.Help.Render("Press 'q' to quit"))
		return content.String()
	}

	var content strings.Builder

	titleText := "Music Library"
	if m.currentSong != nil {
		playIcon := "⏸"
		if m.isPlaying {
			playIcon = "▶"
		}
		titleText += fmt.Sprintf(" %s %s", playIcon, m.currentSong.Title)
	}
	content.WriteString(currentStyles.Title.Render(titleText))
	content.WriteString("\n\n")

	visibleHeight := max(m.height-6, 5)

	start := 0
	end := len(m.songs)

	if len(m.songs) > visibleHeight {
		if m.cursor >= visibleHeight/2 {
			start = min(m.cursor-visibleHeight/2, len(m.songs)-visibleHeight)
		}
		end = min(start+visibleHeight, len(m.songs))
	}

	for i := start; i < end; i++ {
		song := m.songs[i]

		var songInfo string
		if song.Artist != "" && song.Title != "" {
			songInfo = fmt.Sprintf("%s - %s", song.Artist, song.Title)
		} else if song.Title != "" {
			songInfo = song.Title
		} else {

			parts := strings.Split(song.Path, "/")
			if len(parts) > 0 {
				songInfo = parts[len(parts)-1]
			} else {
				songInfo = song.Path
			}
		}

		maxWidth := max(m.width-6, 20)
		if len(songInfo) > maxWidth {
			songInfo = songInfo[:maxWidth-3] + "..."
		}

		prefix := "  "
		isCurrentSong := m.currentSong != nil && song.Path == m.currentSong.Path
		if isCurrentSong {
			prefix = "♪ "
		}

		if i == m.cursor {
			if !isCurrentSong {
				prefix = ""
			}
			content.WriteString(currentStyles.Selected.Render(fmt.Sprintf("► %s%s", prefix, songInfo)))
		} else {
			content.WriteString(currentStyles.Normal.Render(fmt.Sprintf("%s%s", prefix, songInfo)))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")

	pageInfo := fmt.Sprintf("Item %d of %d", m.cursor+1, len(m.songs))
	if len(m.songs) > visibleHeight {
		pageInfo += fmt.Sprintf(" (showing %d-%d)", start+1, end)
	}
	helpText := "↑/↓ navigate • Enter select • q quit"

	combinedLine := lipgloss.JoinHorizontal(lipgloss.Left,
		currentStyles.Help.Render(pageInfo),
		strings.Repeat(" ", max(4, m.width-len(pageInfo)-len(helpText)-8)),
		currentStyles.Help.Render(helpText),
	)
	content.WriteString(combinedLine)

	return content.String()
}
