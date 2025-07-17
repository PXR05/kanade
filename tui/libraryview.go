package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	lib "gmp/library"

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

type LibraryModel struct {
	songs         []lib.Song
	filteredSongs []lib.Song
	cursor        int
	width         int
	height        int
	styles        LibraryStyles
	currentSong   *lib.Song
	isPlaying     bool
	dominantColor string
	searchMode    bool
	searchQuery   string
	showHelp      bool
	position      time.Duration
	totalDuration time.Duration

	groupingMode   GroupingMode
	groups         []GroupItem
	displayItems   []ListItem
	expandedGroups map[string]bool
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

func NewLibraryModel(songs []lib.Song) *LibraryModel {
	model := &LibraryModel{
		songs:          songs,
		filteredSongs:  songs,
		cursor:         0,
		styles:         DefaultLibraryStyles(),
		dominantColor:  DefaultAccentColor,
		searchMode:     false,
		searchQuery:    "",
		showHelp:       false,
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
			if m.groupingMode == GroupByArtist {

				if songs[i].Album != songs[j].Album {
					return songs[i].Album < songs[j].Album
				}
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
		for i, song := range m.filteredSongs {
			m.displayItems = append(m.displayItems, ListItem{
				IsGroup:   false,
				Song:      &song,
				SongIndex: i,
			})
		}
	} else {
		m.groups = m.groupSongs()
		for groupIndex, group := range m.groups {

			m.displayItems = append(m.displayItems, ListItem{
				IsGroup:    true,
				Group:      &group,
				GroupIndex: groupIndex,
			})

			if group.Expanded {
				for songIndex, song := range group.Songs {
					songCopy := song
					m.displayItems = append(m.displayItems, ListItem{
						IsGroup:    false,
						Song:       &songCopy,
						GroupIndex: groupIndex,
						SongIndex:  songIndex,
					})
				}
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
	return tea.Tick(TickInterval, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

func (m *LibraryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case SongSelectedMsg:
		m.currentSong = &msg.Song
		return m, nil

	case DominantColorMsg:
		m.dominantColor = msg.Color
		return m, nil

	case PlaybackStatusMsg:
		m.isPlaying = msg.IsPlaying
		return m, nil

	case PlaybackPositionMsg:
		m.position = msg.Position
		m.totalDuration = msg.TotalDuration
		return m, nil

	case TickMsg:
		return m, tea.Tick(TickInterval, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		})

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchQuery = ""
				m.filteredSongs = m.songs
				m.rebuildDisplayItems()
				m.cursor = 0
				return m, nil
			case "enter":
				m.searchMode = false
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.filterSongs()
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.searchQuery += msg.String()
					m.filterSongs()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "/":
			m.searchMode = true
			m.searchQuery = ""
			return m, nil

		case "enter":
			if len(m.displayItems) > 0 && m.cursor < len(m.displayItems) {
				item := m.displayItems[m.cursor]
				if item.IsGroup {
					m.toggleGroupExpansion()
					return m, nil
				} else {
					return m, func() tea.Msg {
						return SongSelectedMsg{Song: *item.Song}
					}
				}
			}

		case "tab":
			if m.currentSong != nil {
				return m, func() tea.Msg {
					return SwitchViewMsg{View: PlayerView}
				}
			}

		case "g":
			m.toggleGrouping()
			return m, nil

		case "c":
			m.jumpToCurrentSong()
			return m, nil

		case "?":
			m.showHelp = !m.showHelp
			return m, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.displayItems)-1 {
				m.cursor++
			}

		case " ":
			if len(m.displayItems) > 0 && m.cursor < len(m.displayItems) {
				item := m.displayItems[m.cursor]
				if item.IsGroup {

					m.toggleGroupExpansion()
					return m, nil
				} else {

					return m, func() tea.Msg {
						return SongSelectedMsg{Song: *item.Song}
					}
				}
			}

		case "home":
			m.cursor = 0

		case "end":
			m.cursor = len(m.displayItems) - 1
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

func (m *LibraryModel) IsInSearchMode() bool {
	return m.searchMode
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
	availableWidth := SafeMax(m.width-12, TerminalWidthMinimum-20, TerminalWidthMinimum-20)

	if m.groupingMode != NoGrouping {
		availableWidth -= 2
	}

	switch layout {
	case ThreeColumn:

		titleWidth := SafeMax(int(float64(availableWidth)*0.4), 15, 15)
		artistWidth := SafeMax(int(float64(availableWidth)*0.3), 12, 12)
		albumWidth := SafeMax(availableWidth-titleWidth-artistWidth-4, 12, 12)
		return titleWidth, artistWidth, albumWidth
	case TwoColumn:

		titleWidth := SafeMax(int(float64(availableWidth)*0.6), ContentMinWidth, ContentMinWidth)
		artistWidth := SafeMax(availableWidth-titleWidth-DefaultPadding, 15, 15)
		return titleWidth, artistWidth, 0
	default:
		return 0, 0, 0
	}
}

func (m *LibraryModel) formatSongColumns(song *lib.Song, titleWidth, artistWidth, albumWidth int) string {
	layout := m.getColumnLayout()

	switch layout {
	case ThreeColumn:
		title := song.Title
		if title == "" {
			title = ExtractFileName(song.Path)
		}

		artist := song.Artist
		if artist == "" {
			artist = "Unknown Artist"
		}

		album := song.Album
		if album == "" {
			album = "Unknown Album"
		}

		titleCol := PadText(title, titleWidth)
		artistCol := PadText(artist, artistWidth)
		albumCol := PadText(album, albumWidth)

		return fmt.Sprintf("%s  %s  %s", titleCol, artistCol, albumCol)

	case TwoColumn:
		title := song.Title
		if title == "" {
			title = ExtractFileName(song.Path)
		}

		artist := song.Artist
		if artist == "" {
			artist = "Unknown Artist"
		}

		titleCol := PadText(title, titleWidth)
		artistCol := PadText(artist, artistWidth)

		return fmt.Sprintf("%s  %s", titleCol, artistCol)

	default:

		return FormatSongInfo(song.Artist, song.Title, song.Path)
	}
}

func (m *LibraryModel) formatColumnHeaders(titleWidth, artistWidth, albumWidth int) string {
	layout := m.getColumnLayout()

	switch layout {
	case ThreeColumn:
		title := PadText("TITLE", titleWidth)
		artist := PadText("ARTIST", artistWidth)
		album := PadText("ALBUM", albumWidth)
		return fmt.Sprintf("%s  %s  %s", title, artist, album)

	case TwoColumn:
		title := PadText("TITLE", titleWidth)
		artist := PadText("ARTIST", artistWidth)
		return fmt.Sprintf("%s  %s", title, artist)

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

func (m *LibraryModel) renderLibraryContent(currentStyles LibraryStyles, availableHeight int) string {
	var libraryContent strings.Builder

	if len(m.displayItems) == 0 {
		emptyText := ""
		if m.searchQuery != "" {
			emptyText = "No songs match your search\nPress Esc to clear search"
		} else {
			emptyText = "No songs found"
		}

		emptyHeight := SafeMax(availableHeight-BorderAccountWidth, KernelSize, KernelSize)
		topPadding := emptyHeight / DefaultPadding
		for i := 0; i < topPadding; i++ {
			libraryContent.WriteString("\n")
		}

		emptyStyle := lipgloss.NewStyle().
			Width(m.width - BorderAccountWidth).
			Align(lipgloss.Center)
		libraryContent.WriteString(emptyStyle.Render(currentStyles.Normal.Render(emptyText)))

		for i := topPadding + 1; i < emptyHeight; i++ {
			libraryContent.WriteString("\n")
		}
		return libraryContent.String()
	}

	itemsAvailableHeight := availableHeight - 2
	headerLines := 0

	if m.groupingMode == NoGrouping && m.getColumnLayout() != SingleColumn {
		headerLines = 2
		titleWidth, artistWidth, albumWidth := m.calculateColumnWidths()
		headers := m.formatColumnHeaders(titleWidth, artistWidth, albumWidth)
		headerStyle := lipgloss.NewStyle()
		libraryContent.WriteString(headerStyle.Render(currentStyles.Header.Render(headers)))
		libraryContent.WriteString("\n")

		separatorWidth := SafeMax(m.width-BorderAccountWidth, ContentMinWidth, ContentMinWidth)
		separator := strings.Repeat("─", separatorWidth)
		separatorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.getBorderColor()))
		libraryContent.WriteString(separatorStyle.Render(separator))
		libraryContent.WriteString("\n")
	}

	visibleHeight := SafeMax(itemsAvailableHeight-headerLines, MinVisibleHeight, MinVisibleHeight)

	start := 0
	end := len(m.displayItems)

	if len(m.displayItems) > visibleHeight {
		start, end = CalculateVisibleRange(len(m.displayItems), visibleHeight, m.cursor)
	}

	for i := start; i < end; i++ {
		item := m.displayItems[i]

		if item.IsGroup {
			expandIcon := "▶"
			if item.Group.Expanded {
				expandIcon = "▼"
			}

			groupText := fmt.Sprintf("%s %s (%d songs)", expandIcon, item.Group.Name, item.Group.SongCount)
			maxWidth := SafeMax(m.width-BorderAccountWidth, ContentMinWidth, ContentMinWidth)
			if m.groupingMode != NoGrouping {
				maxWidth -= 2
			}
			groupText = TruncateString(groupText, maxWidth)

			var itemStyle lipgloss.Style
			if m.groupingMode != NoGrouping {
				itemStyle = lipgloss.NewStyle().Padding(0, 1)
			} else {
				itemStyle = lipgloss.NewStyle()
			}
			if i == m.cursor {
				libraryContent.WriteString(itemStyle.Render(currentStyles.Selected.Render(groupText)))
			} else {
				libraryContent.WriteString(itemStyle.Render(currentStyles.Group.Render(groupText)))
			}
		} else {
			song := item.Song

			var songDisplay string
			var prefix string

			isCurrentSong := m.currentSong != nil && song.Path == m.currentSong.Path

			if m.groupingMode == NoGrouping {
				layout := m.getColumnLayout()

				if layout == SingleColumn {
					songInfo := FormatSongInfo(song.Artist, song.Title, song.Path)
					maxWidth := SafeMax(m.width-BorderAccountWidth-4, ContentMinWidth, ContentMinWidth)
					if m.groupingMode != NoGrouping {
						maxWidth -= 2
					}
					songDisplay = TruncateString(songInfo, maxWidth)

					if isCurrentSong {
						prefix = "♪ "
					} else {
						prefix = "  "
					}
				} else {
					titleWidth, artistWidth, albumWidth := m.calculateColumnWidths()
					songDisplay = m.formatSongColumns(song, titleWidth, artistWidth, albumWidth)

					if isCurrentSong {
						prefix = "♪ "
					} else {
						prefix = "  "
					}
				}
			} else {
				songInfo := FormatSongInfo(song.Artist, song.Title, song.Path)
				maxWidth := SafeMax(m.width-BorderAccountWidth-4, ContentMinWidth, ContentMinWidth)
				if m.groupingMode != NoGrouping {
					maxWidth -= 2
				}
				songDisplay = TruncateString(songInfo, maxWidth)

				if isCurrentSong {
					prefix = "♪ "
				} else {
					prefix = "  "
				}
			}

			var itemStyle lipgloss.Style
			if m.groupingMode != NoGrouping {
				itemStyle = lipgloss.NewStyle().Padding(0, 1)
			} else {
				itemStyle = lipgloss.NewStyle()
			}
			if i == m.cursor {
				libraryContent.WriteString(itemStyle.Render(currentStyles.Selected.Render(fmt.Sprintf("%s%s", prefix, songDisplay))))
			} else {
				style := currentStyles.Normal
				if m.groupingMode != NoGrouping {
					style = currentStyles.GroupSong
				}
				libraryContent.WriteString(itemStyle.Render(style.Render(fmt.Sprintf("%s%s", prefix, songDisplay))))
			}
		}
		libraryContent.WriteString("\n")
	}

	currentLibraryLines := strings.Count(libraryContent.String(), "\n")
	neededLines := availableHeight - 2
	for i := currentLibraryLines; i < neededLines; i++ {
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
		content.WriteString(currentStyles.Title.Render("GMP"))
		content.WriteString("\n\n")
		content.WriteString(currentStyles.Normal.Render("No songs found"))
		content.WriteString("\n")
		content.WriteString(currentStyles.Help.Render("Add .mp3 or .wav files to the assets folder"))
		content.WriteString("\n\n")
		content.WriteString(currentStyles.Help.Render("Press 'q' to quit"))
		return content.String()
	}

	if m.searchMode {
		searchLine := fmt.Sprintf("%s%s", DefaultSearchPrompt, m.searchQuery)
		content.WriteString(currentStyles.Title.Render(searchLine))
	} else {
		titleText := "GMP"
		content.WriteString(currentStyles.Title.Render(titleText))
	}
	content.WriteString("\n")

	if !m.searchMode && m.searchQuery != "" {
		searchInfo := fmt.Sprintf("Filtered by: %s (Press / to search again, Esc to clear)", m.searchQuery)
		content.WriteString(currentStyles.Help.Render(searchInfo))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	if len(m.displayItems) == 0 {
		emptyContent := lipgloss.NewStyle().Padding(0, DefaultPadding)
		if m.searchQuery != "" {
			content.WriteString(emptyContent.Render(currentStyles.Normal.Render("No songs match your search")))
			content.WriteString("\n")
			content.WriteString(emptyContent.Render(currentStyles.Help.Render("Press Esc to clear search")))
		} else {
			content.WriteString(emptyContent.Render(currentStyles.Normal.Render("No songs found")))
		}
		content.WriteString("\n\n")
		content.WriteString(emptyContent.Render(currentStyles.Help.Render("Press 'q' to quit")))
		return content.String()
	}

	currentHeight := strings.Count(content.String(), "\n") + 1
	availableHeight := m.height - currentHeight - HelpBottomReserve

	libraryTitle := fmt.Sprintf("Library (%d songs)", len(m.filteredSongs))
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true).
		Padding(0, DefaultPadding)
	content.WriteString(titleStyle.Render(libraryTitle))
	content.WriteString("\n")

	libraryContent := m.renderLibraryContent(currentStyles, availableHeight-DefaultPadding)

	borderColor := m.getBorderColor()

	borderedLibrary := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(m.width-BorderAccountWidth).
		Height(availableHeight-DefaultPadding).
		Margin(0, DefaultPadding)

	content.WriteString(borderedLibrary.Render(libraryContent))

	content.WriteString("\n")

	var bottomLine string

	helpStyle := lipgloss.NewStyle().Padding(0, MinimumPadding)

	if (m.showHelp && !m.searchMode) || m.searchMode {
		var helpText string
		if m.searchMode {
			helpText = "Type to search • Enter confirm • Esc cancel"
		} else {
			helpText = BuildHelpText(NavigationKeys, ActionKeys)
		}
		bottomLine = helpStyle.Render(currentStyles.Help.Render(helpText))
	} else {
		var leftContent string
		if m.currentSong != nil {
			songText := fmt.Sprintf("♪ %s", m.currentSong.Title)
			if m.currentSong.Artist != "" {
				songText = fmt.Sprintf("♪ %s - %s", m.currentSong.Artist, m.currentSong.Title)
			}

			if m.totalDuration > 0 {
				positionText := fmt.Sprintf("%s / %s", FormatDuration(m.position), FormatDuration(m.totalDuration))
				songText += fmt.Sprintf(" [%s]", positionText)
			}

			leftContent = songText
		} else {
			leftContent = "No song playing"
		}

		pageInfo := fmt.Sprintf("Item %d of %d", m.cursor+1, len(m.displayItems))

		groupingText := m.getGroupingModeText()
		pageInfo += fmt.Sprintf(" • %s", groupingText)

		if m.groupingMode == NoGrouping {
			layout := m.getColumnLayout()
			var layoutText string
			switch layout {
			case ThreeColumn:
				layoutText = "3 Columns"
			case TwoColumn:
				layoutText = "2 Columns"
			case SingleColumn:
				layoutText = "1 Column"
			}
			pageInfo += fmt.Sprintf(" • %s", layoutText)
		}

		rightContent := pageInfo

		spacedContent := JoinHorizontalWithSpacing(leftContent, rightContent, m.width-BorderAccountWidth)
		bottomLine = helpStyle.Render(currentStyles.Help.Render(spacedContent))
	}

	content.WriteString(bottomLine)

	return content.String()
}
