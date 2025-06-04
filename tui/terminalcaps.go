package tui

import (
	"os"
	"strings"
)

// ImageProtocol represents different terminal image display protocols
type ImageProtocol int

const (
	ProtocolASCII ImageProtocol = iota
	ProtocolSIXEL
	ProtocolITerm2
	ProtocolKitty
	ProtocolTerminology
)

// TerminalCapabilities holds information about terminal image support
type TerminalCapabilities struct {
	SupportedProtocols []ImageProtocol
	BestProtocol       ImageProtocol
	TerminalName       string
	ColorSupport       int
}

// DetectTerminalCapabilities analyzes the current terminal and returns its capabilities
func DetectTerminalCapabilities() *TerminalCapabilities {
	caps := &TerminalCapabilities{
		SupportedProtocols: []ImageProtocol{ProtocolASCII}, // ASCII is always supported
		BestProtocol:       ProtocolASCII,
		TerminalName:       getTerminalName(),
		ColorSupport:       getColorSupport(),
	}

	// Check for specific terminal types and their capabilities
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	term := strings.ToLower(os.Getenv("TERM"))

	switch {
	case termProgram == "iterm.app":
		// iTerm2 supports its own protocol
		caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolITerm2)
		caps.BestProtocol = ProtocolITerm2

	case termProgram == "kitty":
		// Kitty terminal supports its own graphics protocol
		caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolKitty)
		caps.BestProtocol = ProtocolKitty

	case termProgram == "terminology":
		// Terminology supports inline images
		caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolTerminology)
		caps.BestProtocol = ProtocolTerminology

	case strings.Contains(term, "xterm"):
		// Many xterm variants support SIXEL
		if checkSIXELSupport() {
			caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolSIXEL)
			caps.BestProtocol = ProtocolSIXEL
		}

	case strings.Contains(term, "mintty"):
		// Windows Terminal (mintty) often supports SIXEL
		if checkSIXELSupport() {
			caps.SupportedProtocols = append(caps.SupportedProtocols, ProtocolSIXEL)
			caps.BestProtocol = ProtocolSIXEL
		}

	case term == "screen" || strings.HasPrefix(term, "tmux"):
		// Screen/tmux - check if underlying terminal supports protocols
		// For now, default to ASCII but could be enhanced
		caps.BestProtocol = ProtocolASCII
	}

	return caps
}

// getTerminalName returns a human-readable terminal name
func getTerminalName() string {
	if name := os.Getenv("TERM_PROGRAM"); name != "" {
		return name
	}
	if term := os.Getenv("TERM"); term != "" {
		return term
	}
	return "unknown"
}

// getColorSupport estimates the color support level
func getColorSupport() int {
	term := os.Getenv("TERM")
	colorTerm := os.Getenv("COLORTERM")

	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return 16777216 // 24-bit color
	}

	if strings.Contains(term, "256color") {
		return 256
	}

	if strings.Contains(term, "color") {
		return 16
	}

	return 8 // Basic color support
}

// checkSIXELSupport attempts to detect SIXEL support
func checkSIXELSupport() bool {
	// Check environment variables that indicate SIXEL support
	if os.Getenv("SIXEL") == "1" {
		return true
	}

	// Some terminals set this when SIXEL is available
	if strings.Contains(os.Getenv("TERM"), "sixel") {
		return true
	}

	// Could implement a more sophisticated check by sending a SIXEL probe
	// and checking for a response, but that's complex and risky
	// For now, we'll be conservative and only enable for known terminals

	return false
}

// HasProtocolSupport checks if a specific protocol is supported
func (caps *TerminalCapabilities) HasProtocolSupport(protocol ImageProtocol) bool {
	for _, p := range caps.SupportedProtocols {
		if p == protocol {
			return true
		}
	}
	return false
}

// GetProtocolName returns a human-readable name for the protocol
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
