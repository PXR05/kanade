package tui

import "time"

// UI Layout Constants
const (
	// Terminal size thresholds
	TerminalWidthLarge   = 250
	TerminalWidthMedium  = 200
	TerminalWidthSmall   = 160
	TerminalWidthTiny    = 100
	TerminalWidthMinimum = 80
	TerminalWidthNarrow  = 140

	TerminalHeightLarge   = 70
	TerminalHeightMedium  = 60
	TerminalHeightSmall   = 50
	TerminalHeightTiny    = 40
	TerminalHeightMinimum = 25
	TerminalHeightNarrow  = 30

	// Album art sizing
	AlbumArtMinSize       = 8
	AlbumArtMaxSizeLarge  = 120
	AlbumArtMaxSizeMedium = 100
	AlbumArtMaxSizeSmall  = 80
	AlbumArtMaxSizeTiny   = 60

	// Album art size thresholds
	AlbumArtMediumMin = 50
	AlbumArtMediumMax = 65
	AlbumArtSmallMin  = 40
	AlbumArtSmallMax  = 50
	AlbumArtTinyMin   = 35
	AlbumArtTinyMax   = 35
	AlbumArtMiniMin   = 25
	AlbumArtMiniMax   = 25
	AlbumArtMinMax    = 18

	// UI spacing and padding
	DefaultPadding   = 2
	MinimumPadding   = 1
	DefaultMargin    = 1
	ProgressBarWidth = 40
	VolumeBarWidth   = 20

	// Content sizing
	MaxDisplayItems  = 50
	MinVisibleHeight = 5
	ContentMinWidth  = 20
	ContentMinHeight = 10

	// Layout thresholds
	MinWidthForTwoColumn   = TerminalWidthMinimum
	MinWidthForThreeColumn = TerminalWidthTiny + 20

	// Search and navigation
	DefaultSearchPrompt  = "Search: /"
	MaxSongDisplayLength = 50
)

// Timing Constants
const (
	// Update intervals
	TickInterval     = 200 * time.Millisecond
	SlowTickInterval = 1 * time.Second
	FastTickInterval = 100 * time.Millisecond

	// Timeouts and delays
	VolumeBarTimeout = 2 * time.Second
	ErrorTimeout     = 5 * time.Second
	TrackChangeDelay = 250 * time.Millisecond
	SeekInterval     = 10 * time.Second

	// Playback thresholds
	PlaybackEndThreshold = 100 * time.Millisecond
)

// Color Constants
const (
	// Default colors
	DefaultAccentColor   = "#FAFAFA"
	DefaultErrorColor    = "#FF5555"
	DefaultSuccessColor  = "#04B575"
	DefaultWarningColor  = "#FFB86C"
	DefaultTextColor     = "#FAFAFA"
	DefaultSecondaryText = "#CCCCCC"
	DefaultMutedText     = "#666666"
	DefaultBorderColor   = "#874BFD"

	// Color adjustment factors
	BrightenFactor        = 1.8
	DarkenFactor          = 0.6
	ContrastThresholdLow  = 0.3
	ContrastThresholdHigh = 0.7

	// Brightness thresholds
	MinBrightness = 50.0
	MaxBrightness = 200.0
)

// Album Art Constants
const (
	// Sampling and rendering
	DefaultSampleStep     = 3
	HighQualitySampleStep = 5
	LargeSampleThreshold  = 400000
	SuperSamplingFactor   = 5
	KernelSize            = 3

	// Color processing
	ColorMapSize      = 1024
	AlphaThreshold    = 32768
	ColorQuantizeMask = 0xF0
	MinColorCount     = 0

	// Progress bar rendering
	ProgressBarBlocks = 9
	ProgressBarStep   = 8
)

// Downloader Constants
const (
	MinInputWidth         = 40
	MaxTitleDisplayLength = 40
	ProgressBarMinWidth   = 15
	ProgressBarMaxWidth   = 30
	DefaultVisibleItems   = 10
	TopPaddingLines       = 1
	HelpBottomReserve     = 2
	BorderAccountWidth    = 4
)

