package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type CommandBarStyles struct {
	Container lipgloss.Style
	Prompt    lipgloss.Style
	Input     lipgloss.Style
}

func DefaultCommandBarStyles() CommandBarStyles {
	return CommandBarStyles{
		Prompt: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(DefaultAccentColor)),
		Input: lipgloss.NewStyle().
			Foreground(lipgloss.Color(DefaultTextColor)),
	}
}

type CommandBar struct {
	Active bool
	Input  string
	width  int
	styles CommandBarStyles
	Prompt string
}

func NewCommandBar() *CommandBar {
	return &CommandBar{
		Active: false,
		Input:  "",
		width:  0,
		styles: DefaultCommandBarStyles(),
		Prompt: "/",
	}
}

func (c *CommandBar) UpdateSize(width, _ int) {
	c.width = width
}

func (c *CommandBar) Reset() {
	c.Input = ""
}

func (c *CommandBar) View() string {
	var builder strings.Builder

	prompt := ":"
	if c.Prompt != "" {
		prompt = c.Prompt
	}
	builder.WriteString(c.styles.Prompt.Render(prompt))

	input := c.Input
	if time.Now().UnixMilli()/500%2 == 0 {
		input += "█"
	}
	builder.WriteString(c.styles.Input.Render(input))

	line := builder.String()
	if c.width > 0 {
		line = lipgloss.NewStyle().Width(c.width).Render(line)
	}
	return line
}
