package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "0:00"
	}
	totalSeconds := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", totalSeconds/60, totalSeconds%60)
}

func TruncateString(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	var truncated strings.Builder
	for _, r := range s {
		if lipgloss.Width(truncated.String()+string(r)+"...") > maxWidth {
			break
		}
		truncated.WriteRune(r)
	}
	return truncated.String() + "..."
}

func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func ClampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func PadText(text string, width int) string {
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return TruncateString(text, width)
	}
	return text + strings.Repeat(" ", width-textWidth)
}

func JoinHorizontalWithSpacing(left, right string, totalWidth int) string {
	spacingWidth := max(totalWidth-lipgloss.Width(left)-lipgloss.Width(right)-4, 0)
	return lipgloss.JoinHorizontal(lipgloss.Left,
		left,
		strings.Repeat(" ", spacingWidth),
		right,
	)
}

func RenderProgressBar(width int, progress float64, fillColor string) string {
	if width <= 0 {
		return ""
	}

	blocks := []string{"░", "▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	exactPos := ClampFloat64(progress, 0.0, 1.0) * float64(width)

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(fillColor))
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(Colors.DarkenColor(fillColor, DarkenFactor)))

	var bar strings.Builder
	bar.Grow(width * 16)

	for i := range width {
		charProgress := exactPos - float64(i)

		switch {
		case charProgress <= 0:
			bar.WriteString(emptyStyle.Render("░"))
		case charProgress >= 1.0:
			bar.WriteString(filledStyle.Render("█"))
		default:
			blockIndex := ClampInt(int(charProgress*ProgressBarStep)+1, 1, ProgressBarStep)
			bar.WriteString(filledStyle.Render(blocks[blockIndex]))
		}
	}

	return bar.String()
}

func CalculateVisibleRange(totalItems, visibleHeight, currentIndex int) (start, end int) {
	if totalItems <= visibleHeight {
		return 0, totalItems
	}

	start = 0
	if currentIndex >= visibleHeight/2 {
		start = min(currentIndex-visibleHeight/2, totalItems-visibleHeight)
	}
	end = start + visibleHeight
	return start, end
}

func AppendRunesInput(current string, msg tea.KeyMsg) string {
	if msg.Type == tea.KeySpace {
		return current + " "
	}
	if len(msg.Runes) == 0 {
		return current
	}

	var out strings.Builder
	out.WriteString(current)
	for _, r := range msg.Runes {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func ExtractFileName(path string) string {
	return filepath.Base(path)
}

func FormatSongInfo(artist, title, path string) string {
	switch {
	case artist != "" && title != "":
		return fmt.Sprintf("%s - %s", artist, title)
	case title != "":
		return title
	default:
		return ExtractFileName(path)
	}
}
