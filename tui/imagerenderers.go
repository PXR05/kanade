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

// ImageRenderer interface for different rendering protocols
type ImageRenderer interface {
	RenderImage(img image.Image, width, height int) string
	GetProtocol() ImageProtocol
}

// SIXELRenderer renders images using the SIXEL protocol
type SIXELRenderer struct{}

func (r *SIXELRenderer) GetProtocol() ImageProtocol {
	return ProtocolSIXEL
}

func (s *SIXELRenderer) RenderImage(img image.Image, width, height int) string {
	// Simple SIXEL implementation
	// Note: This is a basic implementation. A full SIXEL encoder would be more complex

	// For now, we'll create a simplified SIXEL that displays a colored block
	// representing the dominant color of the image
	dominantColor := getDominantColor(img)

	// SIXEL format: ESC P q "attributes" data ESC \
	// This creates a simple colored rectangle
	sixelData := fmt.Sprintf("\033Pq\"1;1;%d;%d", width, height)

	// Define color (simplified)
	red, green, blue := dominantColor.R, dominantColor.G, dominantColor.B
	sixelData += fmt.Sprintf("#0;2;%d;%d;%d",
		int(float64(red)*100/255),
		int(float64(green)*100/255),
		int(float64(blue)*100/255))

	// Draw a simple rectangle
	for y := 0; y < height/6; y++ { // SIXEL uses 6-pixel high bands
		sixelData += "#0"
		for x := 0; x < width; x++ {
			sixelData += "?" // Full 6-pixel column
		}
		sixelData += "$" // Carriage return
	}

	sixelData += "\033\\"

	return sixelData
}

// ITerm2Renderer renders images using the iTerm2 inline image protocol
type ITerm2Renderer struct{}

func (r *ITerm2Renderer) GetProtocol() ImageProtocol {
	return ProtocolITerm2
}

func (r *ITerm2Renderer) RenderImage(img image.Image, width, height int) string {
	// Resize image to target dimensions
	resizedImg := resizeImage(img, width, height)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedImg); err != nil {
		return "Error encoding image for iTerm2"
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// iTerm2 inline image format
	// ESC ] 1337 ; File=inline=1;width=<width>;height=<height>:<base64_data> BEL
	return fmt.Sprintf("\033]1337;File=inline=1;width=%d;height=%d:%s\a",
		width, height, encoded)
}

// KittyRenderer renders images using the Kitty graphics protocol
type KittyRenderer struct{}

func (r *KittyRenderer) GetProtocol() ImageProtocol {
	return ProtocolKitty
}

func (r *KittyRenderer) RenderImage(img image.Image, width, height int) string {
	// Resize image to target dimensions
	resizedImg := resizeImage(img, width, height)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedImg); err != nil {
		return "Error encoding image for Kitty"
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Split into chunks (Kitty has size limits)
	const maxChunkSize = 4096
	chunks := splitIntoChunks(encoded, maxChunkSize)

	var result strings.Builder

	for i, chunk := range chunks {
		if i == 0 {
			// First chunk with metadata
			result.WriteString(fmt.Sprintf("\033_Ga=T,f=100,s=%d,v=%d,m=1;%s\033\\",
				width, height, chunk))
		} else if i == len(chunks)-1 {
			// Last chunk
			result.WriteString(fmt.Sprintf("\033_Gm=0;%s\033\\", chunk))
		} else {
			// Middle chunk
			result.WriteString(fmt.Sprintf("\033_Gm=1;%s\033\\", chunk))
		}
	}

	return result.String()
}

// TerminologyRenderer renders images using Terminology's inline image protocol
type TerminologyRenderer struct{}

func (r *TerminologyRenderer) GetProtocol() ImageProtocol {
	return ProtocolTerminology
}

func (r *TerminologyRenderer) RenderImage(img image.Image, width, height int) string {
	// Resize image to target dimensions
	resizedImg := resizeImage(img, width, height)

	// Encode as JPEG (Terminology prefers JPEG)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 80}); err != nil {
		return "Error encoding image for Terminology"
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Terminology inline image format
	return fmt.Sprintf("\033}is#%d;%d;%s\000", width, height, encoded)
}

// Helper functions

// getDominantColor finds the most common color in an image (simplified)
func getDominantColor(img image.Image) struct{ R, G, B uint8 } {
	bounds := img.Bounds()
	var r, g, b uint64
	var count uint64

	// Sample every 10th pixel for performance
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

// resizeImage resizes an image to the specified dimensions (simple nearest neighbor)
func resizeImage(img image.Image, width, height int) image.Image {
	bounds := img.Bounds()
	if bounds.Dx() == width && bounds.Dy() == height {
		return img
	}

	// Create a new image with the target dimensions
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

// splitIntoChunks splits a string into chunks of specified size
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

// CreateImageRenderer creates the appropriate renderer for a protocol
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
		return nil // Will fall back to ASCII
	}
}
