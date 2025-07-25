package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "0:00"
	}

	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func TruncateString(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= KernelSize {
		runes := []rune(s)
		truncated := ""
		for i, r := range runes {
			test := truncated + string(r)
			if lipgloss.Width(test) > maxWidth {
				break
			}
			truncated = test
			if i >= len(runes)-1 {
				break
			}
		}
		return truncated
	}

	runes := []rune(s)
	truncated := ""
	for i, r := range runes {
		test := truncated + string(r) + "..."
		if lipgloss.Width(test) > maxWidth {
			break
		}
		truncated += string(r)
		if i >= len(runes)-1 {
			return truncated
		}
	}
	return truncated + "..."
}

func SafeMax(a, b, min int) int {
	result := max(max(b, a), min)
	return result
}

func SafeMin(a, b, max int) int {
	result := min(min(b, a), max)
	return result
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

func BuildHelpText(keyMaps ...map[string]string) string {
	var parts []string
	for _, keyMap := range keyMaps {
		for key, desc := range keyMap {
			parts = append(parts, fmt.Sprintf("%s %s", key, desc))
		}
	}
	return strings.Join(parts, " • ")
}

func CenterText(text string, width int) string {
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return TruncateString(text, width)
	}
	padding := (width - textWidth) / 2
	return strings.Repeat(" ", padding) + text
}

func PadText(text string, width int) string {
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return TruncateString(text, width)
	}
	return text + strings.Repeat(" ", width-textWidth)
}

func JoinHorizontalWithSpacing(left, right string, totalWidth int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	spacingWidth := SafeMax(DefaultPadding, totalWidth-leftWidth-rightWidth-4, 0)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		left,
		strings.Repeat(" ", spacingWidth),
		right,
	)
}

func CreateProgressBar(width int, progress float64, fillChar, emptyChar rune) string {
	if width <= 0 {
		return ""
	}

	fillWidth := min(int(progress*float64(width)), width)

	var result strings.Builder
	result.Grow(width)

	for range fillWidth {
		result.WriteRune(fillChar)
	}
	for i := fillWidth; i < width; i++ {
		result.WriteRune(emptyChar)
	}

	return result.String()
}

func SplitLongText(text string, maxWidth int) []string {
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= maxWidth {
			currentLine.WriteString(" " + word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

func ExtractFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func FormatSongInfo(artist, title, path string) string {
	if artist != "" && title != "" {
		return fmt.Sprintf("%s - %s", artist, title)
	} else if title != "" {
		return title
	} else {
		return ExtractFileName(path)
	}
}

func CalculateVisibleRange(totalItems, visibleHeight, currentIndex int) (start, end int) {
	if totalItems <= visibleHeight {
		return 0, totalItems
	}

	if currentIndex >= visibleHeight/2 {
		start = SafeMin(currentIndex-visibleHeight/2, totalItems-visibleHeight, totalItems-visibleHeight)
	}
	end = SafeMin(start+visibleHeight, totalItems, totalItems)

	return start, end
}

func StringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func IsValidHexColor(color string) bool {
	if len(color) != 7 && len(color) != 4 {
		return false
	}
	if color[0] != '#' {
		return false
	}

	for i := 1; i < len(color); i++ {
		c := color[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
