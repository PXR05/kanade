package tui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
)

type ImageRenderer interface {
	RenderImage(img image.Image, width, height int) string
	GetProtocol() ImageProtocol
}

type SIXELRenderer struct{}

func (r *SIXELRenderer) GetProtocol() ImageProtocol {
	return ProtocolSIXEL
}

func (s *SIXELRenderer) RenderImage(img image.Image, width, height int) string {

	dominantColor := getDominantColor(img)

	sixelData := fmt.Sprintf("\033Pq\"1;1;%d;%d", width, height)

	red, green, blue := dominantColor.R, dominantColor.G, dominantColor.B
	sixelData += fmt.Sprintf("#0;2;%d;%d;%d",
		int(float64(red)*100/255),
		int(float64(green)*100/255),
		int(float64(blue)*100/255))

	for y := 0; y < height/6; y++ {
		sixelData += "#0"
		for x := 0; x < width; x++ {
			sixelData += "?"
		}
		sixelData += "$"
	}

	sixelData += "\033\\"

	return sixelData
}

type ITerm2Renderer struct{}

func (r *ITerm2Renderer) GetProtocol() ImageProtocol {
	return ProtocolITerm2
}

func (r *ITerm2Renderer) RenderImage(img image.Image, width, height int) string {

	resizedImg := resizeImage(img, width, height)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedImg); err != nil {
		return "Error encoding image for iTerm2"
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	return fmt.Sprintf("\033]1337;File=inline=1;width=%d;height=%d:%s\a",
		width, height, encoded)
}

type KittyRenderer struct{}

func (r *KittyRenderer) GetProtocol() ImageProtocol {
	return ProtocolKitty
}

func (r *KittyRenderer) RenderImage(img image.Image, width, height int) string {

	resizedImg := resizeImage(img, width, height)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedImg); err != nil {
		return "Error encoding image for Kitty"
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	const maxChunkSize = 4096
	chunks := splitIntoChunks(encoded, maxChunkSize)

	var result strings.Builder

	for i, chunk := range chunks {
		if i == 0 {

			result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,s=%d,v=%d,m=1;%s\033\\",
				width, height, chunk))
		} else if i == len(chunks)-1 {

			result.WriteString(fmt.Sprintf("\033_Gm=0;%s\033\\", chunk))
		} else {

			result.WriteString(fmt.Sprintf("\033_Gm=1;%s\033\\", chunk))
		}
	}

	return result.String()
}

type TerminologyRenderer struct{}

func (r *TerminologyRenderer) GetProtocol() ImageProtocol {
	return ProtocolTerminology
}

func (r *TerminologyRenderer) RenderImage(img image.Image, width, height int) string {

	resizedImg := resizeImage(img, width, height)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 80}); err != nil {
		return "Error encoding image for Terminology"
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	return fmt.Sprintf("\033}is#%d;%d;%s\000", width, height, encoded)
}

func getDominantColor(img image.Image) struct{ R, G, B uint8 } {
	bounds := img.Bounds()
	var r, g, b uint64
	var count uint64

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			color := img.At(x, y)
			nr, ng, nb, _ := color.RGBA()
			r += uint64(nr >> 8)
			g += uint64(ng >> 8)
			b += uint64(nb >> 8)
			count++
		}
	}

	if count == 0 {
		return struct{ R, G, B uint8 }{128, 128, 128}
	}

	return struct{ R, G, B uint8 }{
		uint8(r / count),
		uint8(g / count),
		uint8(b / count),
	}
}

func resizeImage(img image.Image, width, height int) image.Image {
	bounds := img.Bounds()
	if bounds.Dx() == width && bounds.Dy() == height {
		return img
	}

	newImg := image.NewRGBA(image.Rect(0, 0, width, height))

	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := int(float64(x) * scaleX)
			srcY := int(float64(y) * scaleY)

			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			if srcY >= bounds.Max.Y {
				srcY = bounds.Max.Y - 1
			}

			newImg.Set(x, y, img.At(srcX, srcY))
		}
	}

	return newImg
}

func splitIntoChunks(s string, chunkSize int) []string {
	if len(s) <= chunkSize {
		return []string{s}
	}

	var chunks []string
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}

	return chunks
}

func CreateImageRenderer(protocol ImageProtocol) ImageRenderer {
	switch protocol {
	case ProtocolSIXEL:
		return &SIXELRenderer{}
	case ProtocolITerm2:
		return &ITerm2Renderer{}
	case ProtocolKitty:
		return &KittyRenderer{}
	case ProtocolTerminology:
		return &TerminologyRenderer{}
	default:
		return nil
	}
}
