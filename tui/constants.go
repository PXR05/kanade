package tui

import "time"

const (
	TerminalWidthMedium  = 200
	TerminalWidthSmall   = 160
	TerminalWidthTiny    = 100
	TerminalWidthMinimum = 80
	TerminalWidthNarrow  = 140

	TerminalHeightMedium  = 60
	TerminalHeightSmall   = 50
	TerminalHeightTiny    = 40
	TerminalHeightMinimum = 25
	TerminalHeightNarrow  = 30
)

const (
	AlbumArtMinSize      = 8
	AlbumArtMaxSizeSmall = 80

	AlbumArtMediumMin = 50
	AlbumArtMediumMax = 65
	AlbumArtSmallMin  = 40
	AlbumArtSmallMax  = 50
	AlbumArtTinyMin   = 35
	AlbumArtTinyMax   = 35
	AlbumArtMiniMin   = 25
	AlbumArtMiniMax   = 25
	AlbumArtMinMax    = 18
)

const (
	DefaultPadding   = 2
	MinimumPadding   = 1
	ProgressBarWidth = 40
	VolumeBarWidth   = 20

	MinVisibleHeight = 5
	ContentMinWidth  = 20
	ContentMinHeight = 10

	MinWidthForTwoColumn   = TerminalWidthMinimum
	MinWidthForThreeColumn = TerminalWidthTiny + 20
)

const (
	TickInterval = 100 * time.Millisecond

	StatusTimeout    = 3 * time.Second
	VolumeBarTimeout = 2 * time.Second
	SeekInterval     = 10 * time.Second
)

const (
	VolumeStep = 0.1
)

const (
	DefaultAccentColor   = "#FAFAFA"
	DefaultErrorColor    = "#FF5555"
	DefaultSuccessColor  = "#04B575"
	DefaultWarningColor  = "#FFB86C"
	DefaultTextColor     = "#FAFAFA"
	DefaultSecondaryText = "#CCCCCC"
	DefaultMutedText     = "#666666"
	DefaultBorderColor   = "#874BFD"

	BrightenFactor        = 1.8
	DarkenFactor          = 0.6
	ContrastThresholdLow  = 0.3
	ContrastThresholdHigh = 0.7

	MinBrightness = 50.0
	MaxBrightness = 200.0
)

const (
	DefaultSampleStep     = 3
	HighQualitySampleStep = 5
	LargeSampleThreshold  = 400000
	SuperSamplingFactor   = 3
	KernelSize            = 3

	ColorMapSize      = 1024
	AlphaThreshold    = 32768
	ColorQuantizeMask = 0xF0
	MinColorCount     = 0

	ProgressBarBlocks = 9
	ProgressBarStep   = 8
)

const (
	TopPaddingLines    = 1
	HelpBottomReserve  = 2
	BorderAccountWidth = 4
)
