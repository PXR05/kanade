package tui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"

	lib "gmp/library"

	"github.com/charmbracelet/lipgloss"
)

type AlbumArtRenderer struct {
	width  int
	height int
}

func CalculateOptimalSize(terminalWidth, terminalHeight int) (width, height int) {

	availableWidth := terminalWidth - 10
	availableHeight := terminalHeight - 15

	if availableWidth < 20 {
		availableWidth = 20
	}
	if availableHeight < 10 {
		availableHeight = 10
	}

	var artSize int

	if terminalWidth >= 140 && terminalHeight >= 40 {

		artSize = min(availableWidth/3, availableHeight/2)
		artSize = max(artSize, 25)
		artSize = min(artSize, 40)
	} else if terminalWidth >= 100 && terminalHeight >= 30 {

		artSize = min(availableWidth/4, availableHeight/2)
		artSize = max(artSize, 20)
		artSize = min(artSize, 30)
	} else if terminalWidth >= 80 && terminalHeight >= 25 {

		artSize = min(availableWidth/5, availableHeight/3)
		artSize = max(artSize, 15)
		artSize = min(artSize, 22)
	} else {

		artSize = min(availableWidth/6, availableHeight/4)
		artSize = max(artSize, 8)
		artSize = min(artSize, 18)
	}

	return artSize, artSize
}

func NewAlbumArtRenderer(width, height int) *AlbumArtRenderer {
	return &AlbumArtRenderer{
		width:  width,
		height: height,
	}
}

func NewResponsiveAlbumArtRenderer(terminalWidth, terminalHeight int) *AlbumArtRenderer {
	width, height := CalculateOptimalSize(terminalWidth, terminalHeight)
	return NewAlbumArtRenderer(width, height)
}

func (r *AlbumArtRenderer) UpdateSize(width, height int) {
	r.width = width
	r.height = height
}

func (r *AlbumArtRenderer) UpdateSizeResponsive(terminalWidth, terminalHeight int) {
	width, height := CalculateOptimalSize(terminalWidth, terminalHeight)
	r.UpdateSize(width, height)
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
		factor := min(50.0/brightness, 3.0)
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

	img, _, err := image.Decode(bytes.NewReader(song.Picture.Data))
	if err != nil {
		return r.renderError("Failed to decode image")
	}

	return r.imageToHighResASCII(img)
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

func (r *AlbumArtRenderer) imageToHighResASCII(img image.Image) string {
	bounds := img.Bounds()
	originalWidth := bounds.Dx()
	originalHeight := bounds.Dy()

	squareSize := min(originalHeight, originalWidth)

	cropX := (originalWidth - squareSize) / 2
	cropY := (originalHeight - squareSize) / 2

	renderWidth := r.width * 2
	renderHeight := r.height

	sampleWidth := renderWidth * 5
	sampleHeight := renderHeight * 5

	scaleX := float64(squareSize) / float64(sampleWidth)
	scaleY := float64(squareSize) / float64(sampleHeight)

	asciiChars := []rune{
		' ', '░', '▒', '▓', '█',
		'⠀', '⠁', '⠃', '⠇', '⠏', '⠟', '⠿', '⡿', '⣿',
		'·', '∙', '•', '◦', '○', '●', '◯',
		'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█',
		'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█',
		'`', '.', '\'', ',', ':', ';', '"', '~', '-', '_', '=', '+', '*',
		'<', '>', '!', '?', '/', '\\', '|', '(', ')', '[', ']', '{', '}',
		'o', 'c', 'v', 'x', 'z', 'X', 'Y', 'U', 'J', 'C', 'L', 'Q', 'O',
		'0', 'Z', 'm', 'w', 'q', 'p', 'd', 'b', 'k', 'h', 'a',
		'#', '%', '&', '8', 'B', '@', '$', 'M', 'W', 'N', 'H',
	}

	var result strings.Builder

	for y := range renderHeight {
		for x := range renderWidth {

			var r, g, b, count uint32

			for sy := range 5 {
				for sx := range 5 {
					sampleX := x*5 + sx
					sampleY := y*5 + sy

					imgX := cropX + int(float64(sampleX)*scaleX)
					imgY := cropY + int(float64(sampleY)*scaleY)

					for dy := -1; dy <= 1; dy++ {
						for dx := -1; dx <= 1; dx++ {
							finalX := imgX + dx
							finalY := imgY + dy

							if finalX >= cropX && finalX < cropX+squareSize &&
								finalY >= cropY && finalY < cropY+squareSize &&
								finalX >= 0 && finalX < originalWidth &&
								finalY >= 0 && finalY < originalHeight {
								pixel := img.At(finalX, finalY)
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
				}
			}

			if count > 0 {
				r /= count
				g /= count
				b /= count
			}

			brightness := float64(r)*0.2126 + float64(g)*0.7152 + float64(b)*0.0722

			brightness = brightness / 255.0
			if brightness <= 0.0031308 {
				brightness = brightness * 12.92
			} else {
				brightness = 1.055*math.Pow(brightness, 1.0/2.4) - 0.055
			}
			brightness = brightness * 255.0

			charIndex := int(brightness * float64(len(asciiChars)-1) / 255)
			if charIndex >= len(asciiChars) {
				charIndex = len(asciiChars) - 1
			}
			if charIndex < 0 {
				charIndex = 0
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
	return fmt.Sprintf("Using: High-Resolution ASCII Art (%dx%d)", r.width, r.height)
}
