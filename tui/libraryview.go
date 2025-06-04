package tui

import (
	"fmt"
	"strings"

	lib "gmp/library"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LibraryModel represents the library view model
type LibraryModel struct {
	songs  []lib.Song
	cursor int
	width  int
	height int
	styles LibraryStyles
}

// LibraryStyles contains styling for the library view
type LibraryStyles struct {
	Title     lipgloss.Style
	Header    lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style
	Help      lipgloss.Style
	Container lipgloss.Style
}

// NewLibraryModel creates a new library view model
func NewLibraryModel(songs []lib.Song) *LibraryModel {
	return &LibraryModel{
		songs:  songs,
		cursor: 0,
		styles: DefaultLibraryStyles(),
	}
}

// DefaultLibraryStyles returns default styling for the library view
func DefaultLibraryStyles() LibraryStyles {
	return LibraryStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true).
			Margin(1, 0),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EE6FF8")).
			Background(lipgloss.Color("#2A2A2A")).
			Padding(0, 1).
			Bold(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(0, 1),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Margin(1, 0),
		Container: lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")),
	}
}

// Init initializes the library model
func (m *LibraryModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the library view
func (m *LibraryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
				// Send selected song message
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

// View renders the library view
func (m *LibraryModel) View() string {
	if len(m.songs) == 0 {
		return m.styles.Container.Render(
			m.styles.Title.Render("🎵 Music Library") + "\n\n" +
				m.styles.Normal.Render("No songs found in your library.") + "\n" +
				m.styles.Normal.Render("Add some .mp3 or .wav files to the assets folder.") + "\n\n" +
				m.styles.Help.Render("Press 'q' to quit"),
		)
	}

	var content strings.Builder

	// Title
	content.WriteString(m.styles.Title.Render("🎵 Music Library"))
	content.WriteString("\n")
	content.WriteString(m.styles.Header.Render(fmt.Sprintf("Found %d songs", len(m.songs))))
	content.WriteString("\n\n")

	// Calculate visible range for scrolling
	visibleHeight := m.height - 10 // Account for title, header, help text, etc.
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := 0
	end := len(m.songs)

	// Simple scrolling logic
	if len(m.songs) > visibleHeight {
		if m.cursor >= visibleHeight/2 {
			start = m.cursor - visibleHeight/2
			if start > len(m.songs)-visibleHeight {
				start = len(m.songs) - visibleHeight
			}
		}
		end = start + visibleHeight
		if end > len(m.songs) {
			end = len(m.songs)
		}
	}

	// Song list
	for i := start; i < end; i++ {
		song := m.songs[i]

		// Format song info
		var songInfo string
		if song.Artist != "" && song.Title != "" {
			songInfo = fmt.Sprintf("%s - %s", song.Artist, song.Title)
		} else if song.Title != "" {
			songInfo = song.Title
		} else {
			// Fallback to filename
			parts := strings.Split(song.Path, "/")
			if len(parts) > 0 {
				songInfo = parts[len(parts)-1]
			} else {
				songInfo = song.Path
			}
		}

		// Add album info if available
		if song.Album != "" {
			songInfo += fmt.Sprintf(" [%s]", song.Album)
		}

		// Truncate if too long
		maxWidth := m.width - 10
		if maxWidth < 20 {
			maxWidth = 20
		}
		if len(songInfo) > maxWidth {
			songInfo = songInfo[:maxWidth-3] + "..."
		}

		// Apply styling
		if i == m.cursor {
			content.WriteString(m.styles.Selected.Render(fmt.Sprintf("► %s", songInfo)))
		} else {
			content.WriteString(m.styles.Normal.Render(fmt.Sprintf("  %s", songInfo)))
		}
		content.WriteString("\n")
	}

	// Scrolling indicator
	if len(m.songs) > visibleHeight {
		content.WriteString("\n")
		content.WriteString(m.styles.Help.Render(
			fmt.Sprintf("Showing %d-%d of %d songs", start+1, end, len(m.songs))))
	}

	// Help text
	content.WriteString("\n\n")
	content.WriteString(m.styles.Help.Render("Navigation: ↑/↓ or j/k • Select: Enter/Space • Quit: q"))

	return m.styles.Container.Render(content.String())
}
