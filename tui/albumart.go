package tui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // Import for gif support
	_ "image/jpeg" // Import for jpeg support
	_ "image/png"  // Import for png support
	"strings"

	lib "gmp/library"

	"github.com/charmbracelet/lipgloss"
)

// AlbumArtRenderer handles rendering album art using the best available protocol
type AlbumArtRenderer struct {
	width        int
	height       int
	capabilities *TerminalCapabilities
	renderer     ImageRenderer
}

// NewAlbumArtRenderer creates a new album art renderer with terminal detection
func NewAlbumArtRenderer(width, height int) *AlbumArtRenderer {
	caps := DetectTerminalCapabilities()

	var renderer ImageRenderer
	if caps.BestProtocol != ProtocolASCII {
		renderer = CreateImageRenderer(caps.BestProtocol)
	}

	return &AlbumArtRenderer{
		width:        width,
		height:       height,
		capabilities: caps,
		renderer:     renderer,
	}
}

// RenderAlbumArt renders album art using the best available protocol
func (r *AlbumArtRenderer) RenderAlbumArt(song lib.Song) string {
	if song.Picture == nil || song.Picture.Data == nil || len(song.Picture.Data) == 0 {
		return r.renderPlaceholder()
	}

	// Decode the image data
	img, format, err := image.Decode(bytes.NewReader(song.Picture.Data))
	if err != nil {
		return r.renderError(fmt.Sprintf("Failed to decode %s image", song.Picture.MIMEType))
	}

	// Try to render with the best available protocol
	if r.renderer != nil {
		// Convert dimensions to character-based measurements for terminals
		charWidth := r.width * 8    // Approximate pixels per character width
		charHeight := r.height * 16 // Approximate pixels per character height

		result := r.renderer.RenderImage(img, charWidth, charHeight)

		// Add a header showing which protocol is being used
		protocol := GetProtocolName(r.capabilities.BestProtocol)
		header := r.createProtocolHeader(protocol)

		return header + "\n" + result
	}

	// Fallback to ASCII art
	return r.imageToASCII(img, format)
}

// createProtocolHeader creates a small header showing which image protocol is in use
func (r *AlbumArtRenderer) createProtocolHeader(protocolName string) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Italic(true).
		Width(r.width).
		Align(lipgloss.Center)

	return style.Render(fmt.Sprintf("[ %s ]", protocolName))
}

// renderPlaceholder renders a placeholder when no album art is available
func (r *AlbumArtRenderer) renderPlaceholder() string {
	style := lipgloss.NewStyle().
		Width(r.width).
		Height(r.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#626262")).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("#626262"))

	return style.Render("♪\nNo Album Art\n♪")
}

// renderError renders an error message for debugging
func (r *AlbumArtRenderer) renderError(message string) string {
	style := lipgloss.NewStyle().
		Width(r.width).
		Height(r.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF5555")).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("#FF5555"))

	// Truncate message if too long
	if len(message) > r.width-4 {
		message = message[:r.width-7] + "..."
	}

	return style.Render("❌\n" + message)
}

// imageToASCII converts an image to ASCII art (fallback method)
func (r *AlbumArtRenderer) imageToASCII(img image.Image, format string) string {
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	// Calculate scaling factors to fit the desired dimensions
	scaleX := float64(originalWidth) / float64(r.width)
	scaleY := float64(originalHeight) / float64(r.height)

	// ASCII characters ordered by darkness (light to dark)
	asciiChars := []rune{' ', '░', '▒', '▓', '█'}

	var result strings.Builder
	result.WriteString("┌")
	result.WriteString(strings.Repeat("─", r.width))
	result.WriteString("┐\n")

	for y := 0; y < r.height-2; y++ { // -2 for border
		result.WriteString("│")
		for x := 0; x < r.width; x++ {
			// Map ASCII coordinate to image coordinate
			imgX := int(float64(x) * scaleX)
			imgY := int(float64(y) * scaleY)

			// Ensure we don't go out of bounds
			if imgX >= originalWidth {
				imgX = originalWidth - 1
			}
			if imgY >= originalHeight {
				imgY = originalHeight - 1
			}

			// Get pixel color
			pixel := img.At(imgX, imgY)
			gray := color.GrayModel.Convert(pixel).(color.Gray)

			// Map grayscale value to ASCII character
			charIndex := int(gray.Y) * (len(asciiChars) - 1) / 255
			if charIndex >= len(asciiChars) {
				charIndex = len(asciiChars) - 1
			}

			result.WriteRune(asciiChars[charIndex])
		}
		result.WriteString("│\n")
	}

	result.WriteString("└")
	result.WriteString(strings.Repeat("─", r.width))
	result.WriteString("┘")

	// Add header showing it's ASCII fallback
	header := r.createProtocolHeader("ASCII Art (Fallback)")

	// Apply styling
	style := lipgloss.NewStyle().
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Foreground(lipgloss.Color("#FAFAFA"))

	return header + "\n" + style.Render(result.String())
}

// GetTerminalInfo returns information about the detected terminal capabilities
func (r *AlbumArtRenderer) GetTerminalInfo() string {
	protocolList := make([]string, len(r.capabilities.SupportedProtocols))
	for i, protocol := range r.capabilities.SupportedProtocols {
		protocolList[i] = GetProtocolName(protocol)
	}

	return fmt.Sprintf("Terminal: %s\nSupported: %s\nUsing: %s",
		r.capabilities.TerminalName,
		strings.Join(protocolList, ", "),
		GetProtocolName(r.capabilities.BestProtocol))
}
