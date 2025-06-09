package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	lib "gmp/library"

	"github.com/charmbracelet/lipgloss"
)

type AlbumArtRenderer struct {
	width        int
	height       int
	capabilities *TerminalCapabilities
	renderer     ImageRenderer
}

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

func (r *AlbumArtRenderer) ExtractDominantColor(song lib.Song) string {
	if song.Picture == nil || song.Picture.Data == nil || len(song.Picture.Data) == 0 {

		return "#7D56F4"
	}

	img, _, err := image.Decode(bytes.NewReader(song.Picture.Data))
	if err != nil {

		return "#7D56F4"
	}

	dominantColor := getDominantColorAdvanced(img)

	return fmt.Sprintf("#%02X%02X%02X", dominantColor.R, dominantColor.G, dominantColor.B)
}

func getDominantColorAdvanced(img image.Image) struct{ R, G, B uint8 } {
	bounds := img.Bounds()
	colorMap := make(map[uint32]int)

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 5 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 5 {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()

			if a < 32768 || (r+g+b) < 32768 {
				continue
			}

			r8 := uint8((r >> 8) & 0xF0)
			g8 := uint8((g >> 8) & 0xF0)
			b8 := uint8((b >> 8) & 0xF0)

			colorKey := uint32(r8)<<16 | uint32(g8)<<8 | uint32(b8)
			colorMap[colorKey]++
		}
	}

	var maxCount int
	var dominantColor uint32

	for color, count := range colorMap {
		if count > maxCount {
			maxCount = count
			dominantColor = color
		}
	}

	if maxCount == 0 {

		return struct{ R, G, B uint8 }{125, 86, 244}
	}

	r := uint8((dominantColor >> 16) & 0xFF)
	g := uint8((dominantColor >> 8) & 0xFF)
	b := uint8(dominantColor & 0xFF)

	brightness := float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114

	if brightness < 50 {

		factor := 50.0 / brightness
		if factor > 3.0 {
			factor = 3.0
		}
		r = uint8(float64(r) * factor)
		g = uint8(float64(g) * factor)
		b = uint8(float64(b) * factor)
	} else if brightness > 200 {

		factor := 200.0 / brightness
		r = uint8(float64(r) * factor)
		g = uint8(float64(g) * factor)
		b = uint8(float64(b) * factor)
	}

	return struct{ R, G, B uint8 }{r, g, b}
}

func (r *AlbumArtRenderer) RenderAlbumArt(song lib.Song) string {
	if song.Picture == nil || song.Picture.Data == nil || len(song.Picture.Data) == 0 {
		return r.renderPlaceholder()
	}

	img, format, err := image.Decode(bytes.NewReader(song.Picture.Data))
	if err != nil {
		return r.renderError(fmt.Sprintf("Failed to decode %s image", song.Picture.MIMEType))
	}

	if r.renderer != nil {

		charWidth := r.width * 8
		charHeight := r.height * 16

		result := r.renderer.RenderImage(img, charWidth, charHeight)

		protocol := GetProtocolName(r.capabilities.BestProtocol)
		header := r.createProtocolHeader(protocol)

		return header + "\n" + result
	}

	return r.imageToASCII(img, format)
}

func (r *AlbumArtRenderer) createProtocolHeader(protocolName string) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Italic(true).
		Width(r.width).
		Align(lipgloss.Center)

	return style.Render(fmt.Sprintf("[ %s ]", protocolName))
}

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

func (r *AlbumArtRenderer) renderError(message string) string {
	style := lipgloss.NewStyle().
		Width(r.width).
		Height(r.height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF5555")).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("#FF5555"))

	if len(message) > r.width-4 {
		message = message[:r.width-7] + "..."
	}

	return style.Render("❌\n" + message)
}

func (r *AlbumArtRenderer) imageToASCII(img image.Image, format string) string {
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	squareSize := originalWidth
	if originalHeight < originalWidth {
		squareSize = originalHeight
	}

	cropX := (originalWidth - squareSize) / 2
	cropY := (originalHeight - squareSize) / 2

	renderSize := r.width
	if r.height < r.width {
		renderSize = r.height
	}

	renderWidth := renderSize * 2
	renderHeight := renderSize

	scaleX := float64(squareSize) / float64(renderWidth)
	scaleY := float64(squareSize) / float64(renderHeight)

	asciiChars := []rune{' ', '·', '∙', '•', '▪', '▫', '▭', '▬', '░', '▒', '▓', '█'}

	var result strings.Builder

	for y := 0; y < renderHeight; y++ {
		for x := 0; x < renderWidth; x++ {

			imgX := cropX + int(float64(x)*scaleX)
			imgY := cropY + int(float64(y)*scaleY)

			var r, g, b, count uint32

			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					sampleX := imgX + dx
					sampleY := imgY + dy

					if sampleX >= cropX && sampleX < cropX+squareSize &&
						sampleY >= cropY && sampleY < cropY+squareSize &&
						sampleX >= 0 && sampleX < originalWidth &&
						sampleY >= 0 && sampleY < originalHeight {
						pixel := img.At(sampleX, sampleY)
						pr, pg, pb, pa := pixel.RGBA()

						if pa > 0 {
							r += pr >> 8
							g += pg >> 8
							b += pb >> 8
							count++
						}
					}
				}
			}

			if count > 0 {
				r /= count
				g /= count
				b /= count
			}

			brightness := float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114
			charIndex := int(brightness * float64(len(asciiChars)-1) / 255)
			if charIndex >= len(asciiChars) {
				charIndex = len(asciiChars) - 1
			}

			hexColor := fmt.Sprintf("#%02X%02X%02X", uint8(r), uint8(g), uint8(b))
			charStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))

			result.WriteString(charStyle.Render(string(asciiChars[charIndex])))
		}
		result.WriteString("\n")
	}

	return result.String()
}

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
