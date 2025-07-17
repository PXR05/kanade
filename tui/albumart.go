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
	"sync"

	lib "gmp/library"

	"github.com/charmbracelet/lipgloss"
)

var stringBuilderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

var styleCache = sync.Map{}

type AlbumArtRenderer struct {
	width  int
	height int
}

func CalculateOptimalSize(terminalWidth, terminalHeight int) (width, height int) {
	availableWidth := terminalWidth - 10
	availableHeight := terminalHeight - 15

	availableWidth = SafeMax(availableWidth, ContentMinWidth, ContentMinWidth)
	availableHeight = SafeMax(availableHeight, ContentMinHeight, ContentMinHeight)

	var artSize int

	if terminalWidth >= TerminalWidthMedium && terminalHeight >= TerminalHeightMedium {
		artSize = min(availableWidth/2, availableHeight)
		artSize = ClampInt(artSize, AlbumArtMediumMin, AlbumArtMaxSizeSmall)
	} else if terminalWidth >= TerminalWidthSmall && terminalHeight >= TerminalHeightSmall {
		artSize = min(availableWidth/2, (availableHeight*3)/4)
		artSize = ClampInt(artSize, AlbumArtSmallMin, AlbumArtMediumMax)
	} else if terminalWidth >= TerminalWidthNarrow && terminalHeight >= TerminalHeightTiny {
		artSize = min(availableWidth/2, availableHeight/2)
		artSize = ClampInt(artSize, AlbumArtTinyMin, AlbumArtSmallMax)
	} else if terminalWidth >= TerminalWidthTiny && terminalHeight >= TerminalHeightNarrow {
		artSize = min(availableWidth/3, availableHeight/2)
		artSize = ClampInt(artSize, AlbumArtMiniMin, AlbumArtTinyMax)
	} else if terminalWidth >= TerminalWidthMinimum && terminalHeight >= TerminalHeightMinimum {
		artSize = min(availableWidth/4, availableHeight/3)
		artSize = ClampInt(artSize, AlbumArtMinMax, AlbumArtMiniMax)
	} else {
		artSize = min(availableWidth/6, availableHeight/4)
		artSize = ClampInt(artSize, AlbumArtMinSize, AlbumArtMinMax)
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
		return DefaultAccentColor
	}

	img, _, err := image.Decode(bytes.NewReader(song.Picture.Data))
	if err != nil {
		return DefaultAccentColor
	}

	dominantColor := getDominantColorAdvanced(img)

	return fmt.Sprintf("#%02X%02X%02X", dominantColor.R, dominantColor.G, dominantColor.B)
}

func getDominantColorAdvanced(img image.Image) struct{ R, G, B uint8 } {
	bounds := img.Bounds()

	colorMap := make(map[uint32]int, ColorMapSize)

	minX, minY := bounds.Min.X, bounds.Min.Y
	maxX, maxY := bounds.Max.X, bounds.Max.Y

	stepSize := DefaultSampleStep
	if (maxY-minY)*(maxX-minX) > LargeSampleThreshold {
		stepSize = HighQualitySampleStep
	}

	for y := minY; y < maxY; y += stepSize {
		for x := minX; x < maxX; x += stepSize {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()

			if a < AlphaThreshold || (r+g+b) < AlphaThreshold {
				continue
			}

			r8 := uint8((r >> 8) & ColorQuantizeMask)
			g8 := uint8((g >> 8) & ColorQuantizeMask)
			b8 := uint8((b >> 8) & ColorQuantizeMask)

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

	if maxCount == MinColorCount {
		return struct{ R, G, B uint8 }{125, 86, 244}
	}

	r := uint8((dominantColor >> 16) & 0xFF)
	g := uint8((dominantColor >> 8) & 0xFF)
	b := uint8(dominantColor & 0xFF)

	brightness := float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114

	if brightness < MinBrightness {
		factor := min(MinBrightness/brightness, 3.0)
		r = uint8(min(float64(r)*factor, 255))
		g = uint8(min(float64(g)*factor, 255))
		b = uint8(min(float64(b)*factor, 255))
	} else if brightness > MaxBrightness {
		factor := MaxBrightness / brightness
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
		BorderForeground(lipgloss.Color(DefaultErrorColor)).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(DefaultErrorColor))

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

	sampleWidth := renderWidth * SuperSamplingFactor
	sampleHeight := renderHeight * SuperSamplingFactor

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

	asciiStrings := make([]string, len(asciiChars))
	for i, char := range asciiChars {
		asciiStrings[i] = string(char)
	}

	result := stringBuilderPool.Get().(*strings.Builder)
	result.Reset()
	result.Grow(renderHeight * renderWidth * 20)
	defer stringBuilderPool.Put(result)

	cropXPlusSquare := cropX + squareSize
	cropYPlusSquare := cropY + squareSize
	numChars := len(asciiChars)
	numCharsFloat := float64(numChars - 1)

	sampleCoords := make([][2]int, 25)
	kernelCoords := make([][2]int, ProgressBarBlocks)

	for sy := range 5 {
		for sx := range 5 {
			sampleCoords[sy*5+sx] = [2]int{sx, sy}
		}
	}

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			kernelCoords[(dy+1)*KernelSize+(dx+1)] = [2]int{dx, dy}
		}
	}

	for y := range renderHeight {
		for x := range renderWidth {

			var r, g, b, count uint32

			baseX := x * SuperSamplingFactor
			baseY := y * SuperSamplingFactor

			for i := range 25 {
				sx, sy := sampleCoords[i][0], sampleCoords[i][1]
				sampleX := baseX + sx
				sampleY := baseY + sy

				imgX := cropX + int(float64(sampleX)*scaleX)
				imgY := cropY + int(float64(sampleY)*scaleY)

				if imgX >= cropX && imgX < cropXPlusSquare &&
					imgY >= cropY && imgY < cropYPlusSquare {

					for j := range ProgressBarBlocks {
						dx, dy := kernelCoords[j][0], kernelCoords[j][1]
						finalX := imgX + dx
						finalY := imgY + dy

						if finalX >= cropX && finalX < cropXPlusSquare &&
							finalY >= cropY && finalY < cropYPlusSquare &&
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

			charIndex := int(brightness * numCharsFloat)
			if charIndex >= numChars {
				charIndex = numChars - 1
			}
			if charIndex < 0 {
				charIndex = 0
			}

			hexColor := fmt.Sprintf("#%02X%02X%02X", uint8(r), uint8(g), uint8(b))

			var charStyle lipgloss.Style
			if cached, ok := styleCache.Load(hexColor); ok {
				charStyle = cached.(lipgloss.Style)
			} else {
				charStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))
				styleCache.Store(hexColor, charStyle)
			}

			result.WriteString(charStyle.Render(asciiStrings[charIndex]))
		}
		result.WriteString("\n")
	}

	return result.String()
}
