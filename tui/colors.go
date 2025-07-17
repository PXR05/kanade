package tui

import "fmt"

type ColorUtils struct{}

func NewColorUtils() *ColorUtils {
	return &ColorUtils{}
}

func (c *ColorUtils) calculateLuminance(hexColor string) float64 {
	if hexColor[0] == '#' {
		hexColor = hexColor[1:]
	}

	var r, g, b int
	fmt.Sscanf(hexColor, "%02x%02x%02x", &r, &g, &b)

	rNorm := float64(r) / 255.0
	gNorm := float64(g) / 255.0
	bNorm := float64(b) / 255.0

	rLinear := c.gammaCorrect(rNorm)
	gLinear := c.gammaCorrect(gNorm)
	bLinear := c.gammaCorrect(bNorm)

	return 0.2126*rLinear + 0.7152*gLinear + 0.0722*bLinear
}

func (c *ColorUtils) gammaCorrect(value float64) float64 {
	if value <= 0.03928 {
		return value / 12.92
	}
	return ((value + 0.055) / 1.055) * ((value + 0.055) / 1.055)
}

func (c *ColorUtils) AdjustColorForContrast(hexColor string) string {
	luminance := c.calculateLuminance(hexColor)

	if luminance < ContrastThresholdLow {
		return c.BrightenColor(hexColor, BrightenFactor)
	} else if luminance > ContrastThresholdHigh {
		return c.DarkenColor(hexColor, DarkenFactor)
	}

	return hexColor
}

func (c *ColorUtils) BrightenColor(hexColor string, factor float64) string {
	if hexColor[0] == '#' {
		hexColor = hexColor[1:]
	}

	var r, g, b int
	fmt.Sscanf(hexColor, "%02x%02x%02x", &r, &g, &b)

	r = int(float64(r) + (255-float64(r))*(factor-1.0))
	g = int(float64(g) + (255-float64(g))*(factor-1.0))
	b = int(float64(b) + (255-float64(b))*(factor-1.0))

	r = ClampInt(r, 0, 255)
	g = ClampInt(g, 0, 255)
	b = ClampInt(b, 0, 255)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func (c *ColorUtils) DarkenColor(hexColor string, factor float64) string {
	if hexColor[0] == '#' {
		hexColor = hexColor[1:]
	}

	var r, g, b int
	fmt.Sscanf(hexColor, "%02x%02x%02x", &r, &g, &b)

	r = int(float64(r) * factor)
	g = int(float64(g) * factor)
	b = int(float64(b) * factor)

	r = ClampInt(r, 0, 255)
	g = ClampInt(g, 0, 255)
	b = ClampInt(b, 0, 255)

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

var Colors = NewColorUtils()
