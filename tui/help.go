package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	key  string
	desc string
}

type helpSection struct {
	title   string
	entries []helpEntry
}

func helpSections() []helpSection {
	return []helpSection{
		{
			title: "Global",
			entries: []helpEntry{
				{"q / ctrl+c", "quit"},
				{"?", "toggle this help"},
				{"tab", "switch player / last view"},
				{"p", "play / pause"},
				{"+ / -", "volume up / down"},
				{"m", "mute"},
				{"r", "repeat mode (off/all/one)"},
				{"z", "shuffle on/off"},
				{":", "command mode (:help for list)"},
			},
		},
		{
			title: "Library",
			entries: []helpEntry{
				{"j / k, ↑ / ↓", "navigate"},
				{"enter / space", "play song, expand group"},
				{"/", "search"},
				{"g", "grouping mode"},
				{"c", "jump to current song"},
				{"R", "rescan library folder"},
				{"home / end", "first / last item"},
			},
		},
		{
			title: "Player",
			entries: []helpEntry{
				{"space", "play / pause"},
				{"s", "stop"},
				{"← / →, h / l", "seek ±10s"},
				{"shift+← / →", "previous / next track"},
				{"0", "restart track"},
				{"click bar", "seek to position"},
			},
		},
		{
			title: "Commands",
			entries: []helpEntry{
				{":play :pause :stop", "transport"},
				{":next :prev", "track navigation"},
				{":vol <0-100>", "set volume"},
				{":mute", "mute toggle"},
				{":shuffle [on|off]", "random playback"},
				{":repeat [off|all|one]", "repeat mode"},
				{":search <query>", "filter library"},
				{":vl :vp", "go to view"},
				{":reload", "rescan library folder"},
				{":q", "quit"},
			},
		},
	}
}

func RenderHelpOverlay(width, height int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(DefaultBorderColor)).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultTextColor)).
		Bold(true)
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultSuccessColor)).
		Bold(true)
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultAccentColor))
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(DefaultSecondaryText))

	var body strings.Builder
	body.WriteString(titleStyle.Render("Kanade — Keybindings"))
	body.WriteString("\n")
	body.WriteString(strings.Repeat("─", 46))
	body.WriteString("\n\n")

	for i, section := range helpSections() {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(sectionStyle.Render(section.title))
		body.WriteString("\n")

		keyWidth := 0
		for _, e := range section.entries {
			if w := lipgloss.Width(e.key); w > keyWidth {
				keyWidth = w
			}
		}
		for _, e := range section.entries {
			line := "  " + keyStyle.Render(PadText(e.key, keyWidth)) + "  " + descStyle.Render(e.desc)
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	body.WriteString("\n")
	body.WriteString(descStyle.Render("press any key to close"))

	return lipgloss.Place(
		width,
		max(height, 1),
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(body.String()),
	)
}
