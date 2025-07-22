package tui

import (
	"fmt"
	"strings"
	"time"

	"kanade/downloader"
	lib "kanade/library"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FocusMode int

const (
	FocusInput FocusMode = iota
	FocusList
)

type DownloaderModel struct {
	width  int
	height int

	inputValue string
	inputMode  bool
	focusMode  FocusMode

	downloads []*downloader.DownloadItem
	cursor    int

	styles        DownloaderStyles
	showHelp      bool
	errorMsg      string
	errorTimeout  time.Time
	currentSong   *lib.Song
	dominantColor string

	viewportTop   int
	visibleHeight int

	downloaderManager *downloader.DownloadManager
}

type DownloaderStyles struct {
	Title        lipgloss.Style
	InputBox     lipgloss.Style
	InputActive  lipgloss.Style
	ListItem     lipgloss.Style
	Selected     lipgloss.Style
	Status       lipgloss.Style
	Progress     lipgloss.Style
	Error        lipgloss.Style
	Help         lipgloss.Style
	Container    lipgloss.Style
	ProgressBar  lipgloss.Style
	ProgressFill lipgloss.Style
}

func NewDownloaderModel() *DownloaderModel {
	return &DownloaderModel{
		inputValue:    "",
		inputMode:     true,
		focusMode:     FocusInput,
		downloads:     []*downloader.DownloadItem{},
		cursor:        0,
		styles:        DefaultDownloaderStyles(),
		visibleHeight: DefaultVisibleItems,
		viewportTop:   0,
		dominantColor: DefaultAccentColor,
	}
}

func (m *DownloaderModel) SetDownloaderManager(manager *downloader.DownloadManager) {
	m.downloaderManager = manager
}

func DefaultDownloaderStyles() DownloaderStyles {
	return DownloaderStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Bold(true),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(DefaultMutedText)).
			Padding(0, MinimumPadding),
		InputActive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(DefaultAccentColor)).
			Padding(0, MinimumPadding),
		ListItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultSecondaryText)),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Background(lipgloss.Color("#333333")).
			Bold(true),
		Status: lipgloss.NewStyle().
			Bold(true),
		Progress: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultSuccessColor)),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultErrorColor)).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultMutedText)),
		Container: lipgloss.NewStyle(),
		ProgressBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#3C3C3C")).
			Height(1),
		ProgressFill: lipgloss.NewStyle().
			Background(lipgloss.Color(DefaultAccentColor)).
			Height(1),
	}
}

func (m *DownloaderModel) GetColoredStyles(dominantColor string) DownloaderStyles {
	adjustedColor := Colors.AdjustColorForContrast(dominantColor)
	backgroundAdjustedColor := Colors.DarkenColor(adjustedColor, DarkenFactor)

	return DownloaderStyles{
		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Bold(true),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(DefaultMutedText)).
			Padding(0, MinimumPadding),
		InputActive: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(adjustedColor)).
			Padding(0, MinimumPadding),
		ListItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultSecondaryText)),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)).
			Background(lipgloss.Color(backgroundAdjustedColor)).
			Bold(true),
		Status: lipgloss.NewStyle().
			Bold(true),
		Progress: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultErrorColor)).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color(adjustedColor)),
		Container: lipgloss.NewStyle(),
		ProgressBar: lipgloss.NewStyle().
			Background(lipgloss.Color("#3C3C3C")).
			Height(1),
		ProgressFill: lipgloss.NewStyle().
			Background(lipgloss.Color(adjustedColor)).
			Height(1),
	}
}

func (m *DownloaderModel) Init() tea.Cmd {
	return tea.Tick(SlowTickInterval, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

func (m *DownloaderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.visibleHeight = SafeMax(m.height-ProgressBarStep, MinVisibleHeight, MinVisibleHeight)
		return m, nil

	case SongSelectedMsg:
		m.currentSong = &msg.Song
		return m, nil

	case DominantColorMsg:
		m.dominantColor = msg.Color
		return m, nil

	case DownloadProgressMsg:
		m.updateDownloadProgress(msg.Update)
		return m, nil

	case DownloadCompletedMsg:
		m.handleDownloadCompletion(msg.Event)
		return m, nil

	case DownloadAddedMsg:
		m.refreshDownloads()
		return m, nil

	case TickMsg:
		if m.errorMsg != "" && time.Now().After(m.errorTimeout) {
			m.errorMsg = ""
		}

		m.refreshDownloads()

		return m, tea.Tick(SlowTickInterval, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		})

	case tea.KeyMsg:
		if m.inputMode {
			return m.handleInputMode(msg)
		} else {
			return m.handleListMode(msg)
		}
	}

	return m, nil
}

func (m *DownloaderModel) updateDownloadProgress(update downloader.ProgressUpdate) {
	for i, download := range m.downloads {
		if download.ID == update.ID {
			m.downloads[i].Progress = update.Progress
			m.downloads[i].Downloaded = update.Downloaded
			m.downloads[i].Status = update.Status
			if update.ErrorMsg != "" {
				m.downloads[i].ErrorMsg = update.ErrorMsg
			}
			break
		}
	}
}

func (m *DownloaderModel) handleDownloadCompletion(event downloader.CompletionEvent) {
	if event.Error != nil {
		m.setError(event.Error.Error())
	}
	m.refreshDownloads()
}

func (m *DownloaderModel) refreshDownloads() {
	if m.downloaderManager != nil {
		m.downloads = m.downloaderManager.GetDownloads()
		if m.cursor >= len(m.downloads) && len(m.downloads) > 0 {
			m.cursor = len(m.downloads) - 1
		}
		m.adjustViewport()
	}
}

func (m *DownloaderModel) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.inputValue == "" {
			return m, func() tea.Msg {
				return SwitchViewMsg{View: LibraryView}
			}
		}
		m.inputValue = ""
		return m, nil

	case "enter":
		if m.inputValue != "" && m.downloaderManager != nil {
			id, err := m.downloaderManager.AddDownload(m.inputValue)
			if err != nil {
				m.setError(err.Error())
			} else {
				m.inputValue = ""
				return m, func() tea.Msg {
					return DownloadAddedMsg{ID: id, URL: m.inputValue}
				}
			}
		}
		return m, nil

	case "tab":
		if len(m.downloads) > 0 {
			m.inputMode = false
			m.focusMode = FocusList
		}
		return m, nil

	case "backspace":
		if len(m.inputValue) > 0 {
			m.inputValue = m.inputValue[:len(m.inputValue)-1]
		}
		return m, nil

	default:
		if len(msg.String()) == 1 && msg.String() != "\t" {
			m.inputValue += msg.String()
		}
		return m, nil
	}
}

func (m *DownloaderModel) handleListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg {
			return SwitchViewMsg{View: LibraryView}
		}

	case "tab":
		m.inputMode = true
		m.focusMode = FocusInput
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.adjustViewport()
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(m.downloads)-1 {
			m.cursor++
			m.adjustViewport()
		}
		return m, nil

	case "home":
		m.cursor = 0
		m.viewportTop = 0
		return m, nil

	case "end":
		m.cursor = len(m.downloads) - 1
		m.adjustViewport()
		return m, nil

	case "d", "delete":
		if len(m.downloads) > 0 && m.cursor < len(m.downloads) && m.downloaderManager != nil {
			download := m.downloads[m.cursor]
			err := m.downloaderManager.RemoveDownload(download.ID)
			if err != nil {
				m.setError(err.Error())
			}
			m.refreshDownloads()
		}
		return m, nil

	case "c":
		m.clearCompleted()
		return m, nil

	case "r":
		if len(m.downloads) > 0 && m.cursor < len(m.downloads) && m.downloaderManager != nil {
			download := m.downloads[m.cursor]
			if download.Status == downloader.Failed {
				err := m.downloaderManager.RetryDownload(download.ID)
				if err != nil {
					m.setError(err.Error())
				}
			}
		}
		return m, nil

	case "x":
		if len(m.downloads) > 0 && m.cursor < len(m.downloads) && m.downloaderManager != nil {
			download := m.downloads[m.cursor]
			switch download.Status {
			case downloader.InProgress, downloader.Pending:
				err := m.downloaderManager.CancelDownload(download.ID)
				if err != nil {
					m.setError(err.Error())
				}
			case downloader.Cancelled, downloader.Completed, downloader.Failed:
				err := m.downloaderManager.RemoveDownload(download.ID)
				if err != nil {
					m.setError(err.Error())
				}
			}
			m.refreshDownloads()
		}
		return m, nil

	default:
		return m, nil
	}
}

func (m *DownloaderModel) clearCompleted() {
	if m.downloaderManager == nil {
		return
	}

	for _, download := range m.downloads {
		if download.Status == downloader.Completed {
			m.downloaderManager.RemoveDownload(download.ID)
		}
	}
	m.refreshDownloads()
}

func (m *DownloaderModel) adjustViewport() {
	if len(m.downloads) == 0 {
		m.viewportTop = 0
		return
	}

	start, _ := CalculateVisibleRange(len(m.downloads), m.visibleHeight, m.cursor)
	m.viewportTop = start
}

func (m *DownloaderModel) setError(msg string) {
	m.errorMsg = msg
	m.errorTimeout = time.Now().Add(ErrorTimeout)
}

func (m *DownloaderModel) getStatusText(status downloader.Status) (string, lipgloss.Style) {
	switch status {
	case downloader.Pending:
		return "PENDING", m.styles.Status.Foreground(lipgloss.Color(DefaultWarningColor))
	case downloader.InProgress:
		return "DOWNLOADING", m.styles.Status.Foreground(lipgloss.Color(DefaultAccentColor))
	case downloader.Completed:
		return "COMPLETED", m.styles.Status.Foreground(lipgloss.Color(DefaultSuccessColor))
	case downloader.Failed:
		return "FAILED", m.styles.Status.Foreground(lipgloss.Color(DefaultErrorColor))
	case downloader.Cancelled:
		return "CANCELLED", m.styles.Status.Foreground(lipgloss.Color(DefaultMutedText))
	default:
		return "UNKNOWN", m.styles.Status
	}
}

func (m *DownloaderModel) View() string {
	var currentStyles DownloaderStyles
	if m.currentSong != nil {
		currentStyles = m.GetColoredStyles(m.dominantColor)
	} else {
		currentStyles = m.styles
	}

	var content strings.Builder

	for range TopPaddingLines {
		content.WriteString("\n")
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true).
		Padding(0, DefaultPadding)
	content.WriteString(titleStyle.Render("Downloader"))
	content.WriteString("\n\n")

	var inputStyle lipgloss.Style

	inputWidth := SafeMax(m.width-BorderAccountWidth, MinInputWidth, MinInputWidth)

	if m.inputMode {
		inputStyle = currentStyles.InputActive.Width(inputWidth)
	} else {
		inputStyle = currentStyles.InputBox.Width(inputWidth)
	}

	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true).
		Padding(0, DefaultPadding)
	content.WriteString(titleStyle.Render("URL"))
	content.WriteString("\n")

	inputDisplay := m.inputValue
	if m.inputMode && time.Now().UnixMilli()/500%2 == 0 {
		inputDisplay += "█"
	}

	inputContainerStyle := lipgloss.NewStyle().Padding(0, DefaultPadding)
	content.WriteString(inputContainerStyle.Render(inputStyle.Render(inputDisplay)))
	content.WriteString("\n\n")

	if m.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultErrorColor)).
			Bold(true).
			Padding(0, DefaultPadding)
		content.WriteString(errorStyle.Render("Error: " + m.errorMsg))
		content.WriteString("\n\n")
	}

	currentHeight := strings.Count(content.String(), "\n") + 1
	availableHeight := m.height - currentHeight - HelpBottomReserve

	queueTitle := fmt.Sprintf("Downloads (%d total)", len(m.downloads))
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true).
		Padding(0, DefaultPadding)
	content.WriteString(titleStyle.Render(queueTitle))
	content.WriteString("\n")

	queueContent := m.renderQueueContent(currentStyles, availableHeight-DefaultPadding)

	borderColor := m.getBorderColor()

	borderedQueue := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(m.width-BorderAccountWidth).
		Height(availableHeight-DefaultPadding).
		Padding(0, MinimumPadding).
		Margin(0, DefaultPadding)

	content.WriteString(borderedQueue.Render(queueContent))
	content.WriteString("\n")

	content.WriteString(m.renderHelpSection(currentStyles))

	return content.String()
}

func (m *DownloaderModel) renderQueueContent(currentStyles DownloaderStyles, availableHeight int) string {
	var queueContent strings.Builder

	if len(m.downloads) == 0 {
		emptyText := "No YouTube downloads in queue"
		emptyHeight := SafeMax(availableHeight-BorderAccountWidth, KernelSize, KernelSize)

		topPadding := emptyHeight / DefaultPadding
		for i := 0; i < topPadding; i++ {
			queueContent.WriteString("\n")
		}

		emptyStyle := lipgloss.NewStyle().
			Width(m.width - BorderAccountWidth).
			Align(lipgloss.Center)
		queueContent.WriteString(emptyStyle.Render(currentStyles.Help.Render(emptyText)))

		for i := topPadding + 1; i < emptyHeight; i++ {
			queueContent.WriteString("\n")
		}
	} else {
		scrollLines := 0
		if len(m.downloads) > m.visibleHeight {
			scrollLines = DefaultPadding
		}
		itemsHeight := availableHeight - BorderAccountWidth - scrollLines
		itemsToShow := SafeMax(itemsHeight, 1, 1)

		m.visibleHeight = itemsToShow

		start, end := CalculateVisibleRange(len(m.downloads), m.visibleHeight, m.cursor)
		m.viewportTop = start

		for i := start; i < end; i++ {
			download := m.downloads[i]

			statusText, _ := m.getStatusText(download.Status)

			var itemText strings.Builder
			itemText.WriteString(fmt.Sprintf("[%s] ", statusText))

			titleWidth := SafeMax(m.width-30, MaxTitleDisplayLength, MaxTitleDisplayLength)
			itemText.WriteString(TruncateString(download.Title, titleWidth))

			if download.Status == downloader.InProgress {
				progressBarWidth := ClampInt(m.width/4, ProgressBarMinWidth, ProgressBarMaxWidth)
				progressBar := m.generateProgressBar(progressBarWidth, download.Progress)

				var progressText string
				if download.Size > 0 {
					progressText = fmt.Sprintf(" %s %.0f%% (%s/%s)",
						progressBar,
						download.Progress*100,
						downloader.FormatBytes(download.Downloaded),
						downloader.FormatBytes(download.Size))
				} else {
					progressText = fmt.Sprintf(" %s %.0f%% (%s)",
						progressBar,
						download.Progress*100,
						downloader.FormatBytes(download.Downloaded))
				}
				itemText.WriteString(progressText)
			} else if download.Status == downloader.Completed {
				duration := download.CompletedAt.Sub(download.StartTime)
				sizeText := ""
				if download.Size > 0 {
					sizeText = fmt.Sprintf(" (%s)", downloader.FormatBytes(download.Size))
				}
				itemText.WriteString(fmt.Sprintf(" (completed in %s%s)", FormatDuration(duration), sizeText))
			} else if download.Status == downloader.Failed && download.ErrorMsg != "" {
				itemText.WriteString(fmt.Sprintf(" - %s", download.ErrorMsg))
			}

			itemStyle := lipgloss.NewStyle().Padding(0, MinimumPadding)
			if i == m.cursor && !m.inputMode {
				queueContent.WriteString(itemStyle.Render(currentStyles.Selected.Render(itemText.String())))
			} else {
				queueContent.WriteString(itemStyle.Render(currentStyles.ListItem.Render(itemText.String())))
			}
			queueContent.WriteString("\n")
		}

		currentQueueLines := strings.Count(queueContent.String(), "\n")
		neededLines := availableHeight - BorderAccountWidth - scrollLines
		for i := currentQueueLines; i < neededLines; i++ {
			queueContent.WriteString("\n")
		}

		if len(m.downloads) > m.visibleHeight {
			scrollInfo := fmt.Sprintf("Showing %d-%d of %d",
				start+1,
				SafeMin(start+m.visibleHeight, len(m.downloads), len(m.downloads)),
				len(m.downloads))
			scrollStyle := lipgloss.NewStyle().
				Width(m.width - BorderAccountWidth).
				Align(lipgloss.Center)
			queueContent.WriteString(scrollStyle.Render(currentStyles.Help.Render(scrollInfo)))
		}
	}

	return queueContent.String()
}

func (m *DownloaderModel) getBorderColor() string {
	if m.currentSong != nil {
		if !m.inputMode {
			return Colors.AdjustColorForContrast(m.dominantColor)
		}
		return DefaultMutedText
	}

	if !m.inputMode {
		return DefaultAccentColor
	}
	return DefaultMutedText
}

func (m *DownloaderModel) renderHelpSection(currentStyles DownloaderStyles) string {
	helpStyle := lipgloss.NewStyle().Padding(0, DefaultPadding)

	if m.showHelp {
		var helpParts []string
		if m.inputMode {
			helpParts = []string{
				"Enter submit YouTube URL",
				"Esc clear/back",
				"Tab switch to list",
			}
		} else {
			helpParts = []string{
				"↑/↓ navigate",
				"x cancel/remove",
				"d delete",
				"c clear completed",
				"r retry",
				"Tab switch to input",
				"Esc back",
			}
		}
		helpText := "Commands:\n" + strings.Join(helpParts, " • ")
		return helpStyle.Render(currentStyles.Help.Render(helpText))
	}

	var statusText string
	if m.inputMode {
		statusText = "Paste YouTube URL and press Enter • Tab to switch to list"
	} else {
		statusText = "Navigate with ↑/↓ • x to cancel/remove • Tab to switch to input"
	}
	return helpStyle.Render(currentStyles.Help.Render(statusText))
}

func (m *DownloaderModel) generateProgressBar(width int, progress float64) string {
	if width <= 0 {
		return ""
	}

	blocks := []string{"░", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}

	exactPos := progress * float64(width)
	filledColor := m.dominantColor
	if filledColor == "" {
		filledColor = DefaultAccentColor
	}
	emptyColor := Colors.DarkenColor(filledColor, DarkenFactor)

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
			blockIndex := max(min(int(charProgress*ProgressBarStep)+1, ProgressBarStep), 1)
			char = blocks[blockIndex]
			style = filledStyle
		}

		finalBar.WriteString(style.Render(char))
	}

	return finalBar.String()
}
