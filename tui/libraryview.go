package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	lib "kanade/library"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GroupingMode int

const (
	NoGrouping GroupingMode = iota
	GroupByAlbum
	GroupByArtist
)

type GroupItem struct {
	Name      string
	Songs     []lib.Song
	Expanded  bool
	SongCount int
}

type ListItem struct {
	IsGroup    bool
	Group      *GroupItem
	Song       *lib.Song
	GroupIndex int
	SongIndex  int
}

type LibraryStyles struct {
	Title     lipgloss.Style
	Header    lipgloss.Style
	Selected  lipgloss.Style
	Normal    lipgloss.Style
	Help      lipgloss.Style
	Container lipgloss.Style
	Group     lipgloss.Style
	GroupSong lipgloss.Style
}

type ColumnLayout int

const (
	SingleColumn ColumnLayout = iota
	TwoColumn
	ThreeColumn
)

type LibraryModel struct {
	songs         []lib.Song
	filteredSongs []lib.Song
	cursor        int
	width         int
	height        int
	styles        LibraryStyles
	currentSong   *lib.Song
	dominantColor string
	searchQuery   string
	position      time.Duration
	totalDuration time.Duration

	groupingMode   GroupingMode
	groups         []GroupItem
	displayItems   []ListItem
	expandedGroups map[string]bool

	statusText    string
	statusIsError bool
	statusExpiry  time.Time

	repeatMode RepeatMode
	shuffle    bool

	firstItemY    int
	viewportStart int
}

func NewLibraryModel(songs []lib.Song) *LibraryModel {
	model := &LibraryModel{
		songs:          songs,
		filteredSongs:  songs,
		cursor:         0,
		styles:         DefaultLibraryStyles(),
		dominantColor:  DefaultAccentColor,
		searchQuery:    "",
		groupingMode:   NoGrouping,
		expandedGroups: make(map[string]bool),
	}
	model.rebuildDisplayItems()
	return model
}

func DefaultLibraryStyles() LibraryStyles {
	return LibraryStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Bold(true).
			Padding(0, DefaultPadding),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultMutedText)).
			Bold(true).
			Padding(0, DefaultPadding),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Background(lipgloss.Color("#333333")).
			Bold(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultMutedText)).
			Padding(0, DefaultPadding),
		Container: lipgloss.NewStyle().
			Padding(0, DefaultPadding),
		Group: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultSecondaryText)).
			Bold(true),
		GroupSong: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")),
	}
}

func (m *LibraryModel) GetColoredStyles(dominantColor string) LibraryStyles {
	adjustedColor := Colors.AdjustColorForContrast(dominantColor)
	backgroundAdjustedColor := Colors.DarkenColor(adjustedColor, DarkenFactor)

	return LibraryStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Bold(true).
			Padding(0, DefaultPadding),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)).
			Bold(true).
			Padding(0, DefaultPadding),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Background(lipgloss.Color(backgroundAdjustedColor)).
			Bold(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultSecondaryText)),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)).
			Padding(0, DefaultPadding),
		Container: lipgloss.NewStyle().
			Padding(0, DefaultPadding),
		Group: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)).
			Bold(true),
		GroupSong: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BBBBBB")),
	}
}

func (m *LibraryModel) SetSongs(songs []lib.Song) {
	m.songs = songs

	if m.currentSong != nil {
		found := false
		for i := range songs {
			if songs[i].Path == m.currentSong.Path {
				m.currentSong = &songs[i]
				found = true
				break
			}
		}
		if !found {
			m.currentSong = nil
		}
	}

	m.filterSongs()
	m.jumpToCurrentSong()
}

func (m *LibraryModel) filterSongs() {
	if m.searchQuery == "" {
		m.filteredSongs = m.songs
	} else {
		query := strings.ToLower(m.searchQuery)
		var filtered []lib.Song

		for _, song := range m.songs {
			searchText := strings.ToLower(song.Artist + " " + song.Title + " " + song.Album + " " + song.Path)
			if strings.Contains(searchText, query) {
				filtered = append(filtered, song)
			}
		}
		m.filteredSongs = filtered
	}

	m.rebuildDisplayItems()

	if m.cursor >= len(m.displayItems) {
		m.cursor = max(0, len(m.displayItems)-1)
	}
}

func (m *LibraryModel) groupSongs() []GroupItem {
	if m.groupingMode == NoGrouping {
		return nil
	}

	groupMap := make(map[string][]lib.Song)

	for _, song := range m.filteredSongs {
		var groupKey string
		switch m.groupingMode {
		case GroupByAlbum:
			groupKey = song.Album
			if groupKey == "" {
				groupKey = "Unknown Album"
			}
		case GroupByArtist:
			groupKey = song.Artist
			if groupKey == "" {
				groupKey = "Unknown Artist"
			}
		}
		groupMap[groupKey] = append(groupMap[groupKey], song)
	}

	var groups []GroupItem
	for name, songs := range groupMap {
		sort.Slice(songs, func(i, j int) bool {
			if m.groupingMode == GroupByArtist && songs[i].Album != songs[j].Album {
				return songs[i].Album < songs[j].Album
			}
			return songs[i].Title < songs[j].Title
		})

		groups = append(groups, GroupItem{
			Name:      name,
			Songs:     songs,
			Expanded:  m.expandedGroups[name],
			SongCount: len(songs),
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups
}

func (m *LibraryModel) rebuildDisplayItems() {
	m.displayItems = nil

	if m.groupingMode == NoGrouping {
		for i := range m.filteredSongs {
			m.displayItems = append(m.displayItems, ListItem{
				IsGroup:   false,
				Song:      &m.filteredSongs[i],
				SongIndex: i,
			})
		}
		return
	}

	m.groups = m.groupSongs()
	for groupIndex := range m.groups {
		group := &m.groups[groupIndex]
		m.displayItems = append(m.displayItems, ListItem{
			IsGroup:    true,
			Group:      group,
			GroupIndex: groupIndex,
		})

		if group.Expanded {
			for songIndex := range group.Songs {
				m.displayItems = append(m.displayItems, ListItem{
					IsGroup:    false,
					Song:       &group.Songs[songIndex],
					GroupIndex: groupIndex,
					SongIndex:  songIndex,
				})
			}
		}
	}
}

func (m *LibraryModel) toggleGrouping() {
	switch m.groupingMode {
	case NoGrouping:
		m.groupingMode = GroupByAlbum
	case GroupByAlbum:
		m.groupingMode = GroupByArtist
	case GroupByArtist:
		m.groupingMode = NoGrouping
	}
	m.rebuildDisplayItems()
	m.cursor = 0
}

func (m *LibraryModel) toggleGroupExpansion() {
	if len(m.displayItems) == 0 || m.cursor >= len(m.displayItems) {
		return
	}

	item := m.displayItems[m.cursor]
	if !item.IsGroup {
		return
	}

	groupName := item.Group.Name
	m.expandedGroups[groupName] = !m.expandedGroups[groupName]
	m.rebuildDisplayItems()
}

func (m *LibraryModel) jumpToCurrentSong() {
	if m.currentSong == nil {
		return
	}

	songInFilteredList := false
	for _, song := range m.filteredSongs {
		if song.Path == m.currentSong.Path {
			songInFilteredList = true
			break
		}
	}

	if !songInFilteredList && m.searchQuery != "" {
		m.searchQuery = ""
		m.filteredSongs = m.songs
		m.rebuildDisplayItems()
	}

	if m.groupingMode != NoGrouping {
		for _, group := range m.groups {
			for _, song := range group.Songs {
				if song.Path == m.currentSong.Path {
					if !m.expandedGroups[group.Name] {
						m.expandedGroups[group.Name] = true
						m.rebuildDisplayItems()
					}
					break
				}
			}
		}
	}

	for i, item := range m.displayItems {
		if !item.IsGroup && item.Song != nil && item.Song.Path == m.currentSong.Path {
			m.cursor = i
			return
		}
	}
}

func (m *LibraryModel) Init() tea.Cmd {
	return nil
}

func (m *LibraryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case SongSelectedMsg:
		m.currentSong = &msg.Song
		return m, nil

	case PlaybackPositionMsg:
		m.position = msg.Position
		m.totalDuration = msg.TotalDuration
		return m, nil

	case StatusMsg:
		if msg.IsError {
			m.statusText = msg.Text
			m.statusIsError = true
		} else {
			m.statusText = msg.Text
			m.statusIsError = false
		}
		m.statusExpiry = time.Now().Add(StatusTimeout)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			if len(m.displayItems) > 0 && m.cursor < len(m.displayItems) {
				item := m.displayItems[m.cursor]
				if item.IsGroup {
					m.toggleGroupExpansion()
					return m, nil
				}
				song := *item.Song
				return m, func() tea.Msg {
					return SongSelectedMsg{Song: song}
				}
			}

		case "g":
			m.toggleGrouping()
			return m, nil

		case "c":
			m.jumpToCurrentSong()
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.displayItems)-1 {
				m.cursor++
			}

		case "home":
			m.cursor = 0

		case "end":
			m.cursor = max(0, len(m.displayItems)-1)
		}
	}

	return m, nil
}

func (m *LibraryModel) HandleClick(msg tea.MouseMsg) tea.Cmd {
	if len(m.displayItems) == 0 {
		return nil
	}

	idx := m.viewportStart + (msg.Y - m.firstItemY)
	if idx < 0 || idx >= len(m.displayItems) {
		return nil
	}

	m.cursor = idx
	item := m.displayItems[idx]
	if item.IsGroup {
		m.toggleGroupExpansion()
		return nil
	}

	song := *item.Song
	return func() tea.Msg { return SongSelectedMsg{Song: song} }
}

func (m *LibraryModel) getGroupingModeText() string {
	switch m.groupingMode {
	case NoGrouping:
		return "No Grouping"
	case GroupByAlbum:
		return "Grouped by Album"
	case GroupByArtist:
		return "Grouped by Artist"
	default:
		return "Unknown"
	}
}

func (m *LibraryModel) getColumnLayout() ColumnLayout {
	if m.width >= MinWidthForThreeColumn {
		return ThreeColumn
	} else if m.width >= MinWidthForTwoColumn {
		return TwoColumn
	}
	return SingleColumn
}

func (m *LibraryModel) calculateColumnWidths() (int, int, int) {
	layout := m.getColumnLayout()
	availableWidth := max(m.width-12, TerminalWidthMinimum-20)

	if m.groupingMode != NoGrouping {
		availableWidth -= 2
	}

	switch layout {
	case ThreeColumn:
		titleWidth := max(int(float64(availableWidth)*0.4), 15)
		artistWidth := max(int(float64(availableWidth)*0.3), 12)
		albumWidth := max(availableWidth-titleWidth-artistWidth-4, 12)
		return titleWidth, artistWidth, albumWidth
	case TwoColumn:
		titleWidth := max(int(float64(availableWidth)*0.6), ContentMinWidth)
		artistWidth := max(availableWidth-titleWidth-DefaultPadding, 15)
		return titleWidth, artistWidth, 0
	default:
		return 0, 0, 0
	}
}

func (m *LibraryModel) formatSongColumns(song *lib.Song, titleWidth, artistWidth, albumWidth int) string {
	layout := m.getColumnLayout()

	title := song.Title
	if title == "" {
		title = ExtractFileName(song.Path)
	}
	artist := song.Artist
	if artist == "" {
		artist = "Unknown Artist"
	}

	switch layout {
	case ThreeColumn:
		album := song.Album
		if album == "" {
			album = "Unknown Album"
		}
		return fmt.Sprintf("%s  %s  %s",
			PadText(title, titleWidth),
			PadText(artist, artistWidth),
			PadText(album, albumWidth))
	case TwoColumn:
		return fmt.Sprintf("%s  %s",
			PadText(title, titleWidth),
			PadText(artist, artistWidth))
	default:
		return FormatSongInfo(song.Artist, song.Title, song.Path)
	}
}

func (m *LibraryModel) formatColumnHeaders(titleWidth, artistWidth, albumWidth int) string {
	layout := m.getColumnLayout()

	switch layout {
	case ThreeColumn:
		return fmt.Sprintf("%s  %s  %s",
			PadText("TITLE", titleWidth),
			PadText("ARTIST", artistWidth),
			PadText("ALBUM", albumWidth))
	case TwoColumn:
		return fmt.Sprintf("%s  %s",
			PadText("TITLE", titleWidth),
			PadText("ARTIST", artistWidth))
	default:
		return "SONG"
	}
}

func (m *LibraryModel) GetOrderedSongs() []lib.Song {
	if m.groupingMode == NoGrouping {
		return m.filteredSongs
	}

	var orderedSongs []lib.Song
	for _, group := range m.groups {
		orderedSongs = append(orderedSongs, group.Songs...)
	}
	return orderedSongs
}

func (m *LibraryModel) FindSongIndex(targetSong lib.Song) int {
	orderedSongs := m.GetOrderedSongs()
	for i, song := range orderedSongs {
		if song.Path == targetSong.Path {
			return i
		}
	}
	return -1
}

func (m *LibraryModel) getBorderColor() string {
	if m.currentSong != nil {
		return Colors.AdjustColorForContrast(m.dominantColor)
	}
	return DefaultAccentColor
}

func (m *LibraryModel) statusActive() bool {
	return m.statusText != "" && time.Now().Before(m.statusExpiry)
}

func (m *LibraryModel) renderEmptyState(currentStyles LibraryStyles, availableHeight int) string {
	var out strings.Builder

	emptyText := "No songs found\nPress R to rescan the folder"
	if m.searchQuery != "" {
		emptyText = "No songs match your search\nPress Esc to clear search"
	}

	emptyHeight := max(availableHeight-BorderAccountWidth, KernelSize)
	topPadding := emptyHeight / DefaultPadding
	for range topPadding {
		out.WriteString("\n")
	}

	emptyStyle := lipgloss.NewStyle().
		Width(max(m.width-BorderAccountWidth, ContentMinWidth)).
		Align(lipgloss.Center)
	out.WriteString(emptyStyle.Render(currentStyles.Normal.Render(emptyText)))

	for i := topPadding + 1; i < emptyHeight; i++ {
		out.WriteString("\n")
	}
	return out.String()
}

func (m *LibraryModel) renderLibraryContent(currentStyles LibraryStyles, availableHeight int) string {
	var libraryContent strings.Builder

	if len(m.displayItems) == 0 {
		return m.renderEmptyState(currentStyles, availableHeight)
	}

	itemsAvailableHeight := availableHeight - 2
	headerLines := 0

	if m.groupingMode == NoGrouping && m.getColumnLayout() != SingleColumn {
		headerLines = 2
		titleWidth, artistWidth, albumWidth := m.calculateColumnWidths()
		headers := m.formatColumnHeaders(titleWidth, artistWidth, albumWidth)
		libraryContent.WriteString(currentStyles.Header.Render(headers))
		libraryContent.WriteString("\n")

		separatorWidth := max(m.width-BorderAccountWidth, ContentMinWidth)
		separator := strings.Repeat("─", separatorWidth)
		separatorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.getBorderColor()))
		libraryContent.WriteString(separatorStyle.Render(separator))
		libraryContent.WriteString("\n")
	}

	visibleHeight := max(itemsAvailableHeight-headerLines, MinVisibleHeight)

	start := 0
	end := len(m.displayItems)

	if len(m.displayItems) > visibleHeight {
		start, end = CalculateVisibleRange(len(m.displayItems), visibleHeight, m.cursor)
	}

	m.firstItemY = TopPaddingLines + 4 + headerLines
	m.viewportStart = start

	maxContentWidth := max(m.width-BorderAccountWidth, ContentMinWidth)

	for i := start; i < end; i++ {
		item := m.displayItems[i]

		var line string
		var style lipgloss.Style

		if item.IsGroup {
			expandIcon := "▶"
			if item.Group.Expanded {
				expandIcon = "▼"
			}
			text := TruncateString(
				fmt.Sprintf("%s %s (%d songs)", expandIcon, item.Group.Name, item.Group.SongCount),
				maxContentWidth-2)
			style = currentStyles.Group
			line = text
		} else {
			song := item.Song
			isCurrentSong := m.currentSong != nil && song.Path == m.currentSong.Path

			prefix := "  "
			if isCurrentSong {
				prefix = "♪ "
			}

			var body string
			if m.groupingMode == NoGrouping && m.getColumnLayout() != SingleColumn {
				titleWidth, artistWidth, albumWidth := m.calculateColumnWidths()
				body = m.formatSongColumns(song, titleWidth, artistWidth, albumWidth)
			} else {
				body = TruncateString(FormatSongInfo(song.Artist, song.Title, song.Path), maxContentWidth-4)
			}
			line = prefix + body

			if isCurrentSong {
				style = currentStyles.Normal
			} else if m.groupingMode != NoGrouping {
				style = currentStyles.GroupSong
			} else {
				style = currentStyles.Normal
			}
		}

		itemStyle := lipgloss.NewStyle()
		if m.groupingMode != NoGrouping {
			itemStyle = itemStyle.Padding(0, 1)
		}

		if i == m.cursor {
			libraryContent.WriteString(itemStyle.Render(currentStyles.Selected.Render(line)))
		} else {
			libraryContent.WriteString(itemStyle.Render(style.Render(line)))
		}
		libraryContent.WriteString("\n")
	}

	currentLineCount := strings.Count(libraryContent.String(), "\n")
	neededLines := availableHeight - 2
	for i := currentLineCount; i < neededLines; i++ {
		libraryContent.WriteString("\n")
	}

	return libraryContent.String()
}

func (m *LibraryModel) View() string {
	var currentStyles LibraryStyles
	if m.currentSong != nil {
		currentStyles = m.GetColoredStyles(m.dominantColor)
	} else {
		currentStyles = m.styles
	}

	var content strings.Builder

	for range TopPaddingLines {
		content.WriteString("\n")
	}

	if len(m.songs) == 0 {
		content.WriteString(currentStyles.Title.Render("Kanade"))
		content.WriteString("\n\n")
		content.WriteString(currentStyles.Normal.Render("No songs found"))
		content.WriteString("\n")
		content.WriteString(currentStyles.Help.Render(
			"Add audio files (.mp3 .wav .flac .ogg) to the music folder"))
		content.WriteString("\n")
		content.WriteString(currentStyles.Help.Render("Press 'R' to rescan, or 'q' to quit"))
		return content.String()
	}

	content.WriteString(currentStyles.Title.Render("Kanade"))
	content.WriteString("\n\n")

	currentHeight := strings.Count(content.String(), "\n") + 1
	availableHeight := m.height - currentHeight - HelpBottomReserve

	libraryTitle := fmt.Sprintf("Library (%d songs)", len(m.filteredSongs))
	if m.searchQuery != "" {
		libraryTitle = fmt.Sprintf("Library (%d of %d) • %s",
			len(m.filteredSongs), len(m.songs), m.searchQuery)
	}
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true).
		Padding(0, DefaultPadding)
	content.WriteString(titleStyle.Render(libraryTitle))
	content.WriteString("\n")

	libraryContent := m.renderLibraryContent(currentStyles, availableHeight-DefaultPadding)

	borderedLibrary := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.getBorderColor())).
		Width(m.width-BorderAccountWidth).
		Height(availableHeight-DefaultPadding).
		Margin(0, DefaultPadding)

	content.WriteString(borderedLibrary.Render(libraryContent))
	content.WriteString("\n")

	helpStyle := lipgloss.NewStyle().Padding(0, MinimumPadding).Width(m.width)

	var leftContent string
	if m.currentSong != nil {
		songText := fmt.Sprintf("♪ %s", m.currentSong.Title)
		if m.currentSong.Artist != "" {
			songText = fmt.Sprintf("♪ %s - %s", m.currentSong.Artist, m.currentSong.Title)
		}
		if m.totalDuration > 0 {
			songText += fmt.Sprintf(" [%s / %s]",
				FormatDuration(m.position), FormatDuration(m.totalDuration))
		}
		leftContent = songText
	} else {
		leftContent = "No song playing • ? for help"
	}

	rightContent := ""
	switch {
	case m.statusActive():
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(DefaultWarningColor))
		if m.statusIsError {
			statusStyle = statusStyle.Foreground(lipgloss.Color(DefaultErrorColor))
		}
		rightContent = statusStyle.Render(TruncateString(m.statusText, max(m.width/2, ContentMinWidth)))
	default:
		indicators := modeIcon(m.repeatMode, m.shuffle)
		pageInfo := fmt.Sprintf("%d/%d", m.cursor+1, len(m.displayItems))
		if indicators != "" {
			pageInfo += " • " + indicators
		}
		rightContent = pageInfo
	}

	bottomLine := helpStyle.Render(currentStyles.Help.Render(
		JoinHorizontalWithSpacing(leftContent, rightContent, m.width-BorderAccountWidth)))
	content.WriteString(bottomLine)
	content.WriteString("\n")

	return content.String()
}
