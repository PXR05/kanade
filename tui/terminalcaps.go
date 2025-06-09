package tui

import (
	"os"
	"strings"
)

type ImageProtocol int

const (
	ProtocolASCII ImageProtocol = iota
	ProtocolSIXEL
	ProtocolITerm2
	ProtocolKitty
	ProtocolTerminology
)

type TerminalCapabilities struct {
	SupportedProtocols []ImageProtocol
	BestProtocol       ImageProtocol
	TerminalName       string
	ColorSupport       int
}

func DetectTerminalCapabilities() *TerminalCapabilities {
	caps := &TerminalCapabilities{
		SupportedProtocols: []ImageProtocol{ProtocolASCII},
		BestProtocol:       ProtocolASCII,
		TerminalName:       getTerminalName(),
		ColorSupport:       getColorSupport(),
	}

	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	term := strings.ToLower(os.Getenv("TERM"))

	switch {
	case termProgram == "iterm.app":

		caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolITerm2)
		caps.BestProtocol = ProtocolITerm2

	case termProgram == "kitty":

		caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolKitty)
		caps.BestProtocol = ProtocolKitty

	case termProgram == "terminology":

		caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolTerminology)
		caps.BestProtocol = ProtocolTerminology

	case strings.Contains(term, "xterm"):

		if checkSIXELSupport() {
			caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolSIXEL)
			caps.BestProtocol = ProtocolSIXEL
		}

	case strings.Contains(term, "mintty"):

		if checkSIXELSupport() {
			caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolSIXEL)
			caps.BestProtocol = ProtocolSIXEL
		}

	case term == "screen" || strings.HasPrefix(term, "tmux"):

		caps.BestProtocol = ProtocolASCII
	}

	return caps
}

func getTerminalName() string {
	if name := os.Getenv("TERM_PROGRAM"); name != "" {
		return name
	}
	if term := os.Getenv("TERM"); term != "" {
		return term
	}
	return "unknown"
}

func getColorSupport() int {
	term := os.Getenv("TERM")
	colorTerm := os.Getenv("COLORTERM")

	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return 16777216
	}

	if strings.Contains(term, "256color") {
		return 256
	}

	if strings.Contains(term, "color") {
		return 16
	}

	return 8
}

func checkSIXELSupport() bool {

	if os.Getenv("SIXEL") == "1" {
		return true
	}

	if strings.Contains(os.Getenv("TERM"), "sixel") {
		return true
	}

	return false
}

func (caps *TerminalCapabilities) HasProtocolSupport(protocol ImageProtocol) bool {
	for _, p := range caps.SupportedProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

func GetProtocolName(protocol ImageProtocol) string {
	switch protocol {
	case ProtocolASCII:
		return "ASCII Art"
	case ProtocolSIXEL:
		return "SIXEL"
	case ProtocolITerm2:
		return "iTerm2 Images"
	case ProtocolKitty:
		return "Kitty Graphics"
	case ProtocolTerminology:
		return "Terminology Images"
	default:
		return "Unknown"
	}
}
