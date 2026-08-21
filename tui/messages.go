package tui

import (
	"time"

	lib "kanade/library"
)

type ViewState int

const (
	LibraryView ViewState = iota
	PlayerView
)

type SongSelectedMsg struct {
	Song     lib.Song
	KeepView bool
}

type (
	NextTrackMsg    struct{}
	PrevTrackMsg    struct{}
	SongFinishedMsg struct{}
)

type (
	PlayMsg        struct{}
	PauseMsg       struct{}
	TogglePlayMsg  struct{}
	StopRequestMsg struct{}
)

type SwitchViewMsg struct {
	View ViewState
}

type PlaybackStatusMsg struct {
	IsPlaying bool
}

type PlaybackPositionMsg struct {
	Position      time.Duration
	TotalDuration time.Duration
}

type TickMsg struct {
	Time time.Time
}

type ErrorMsg struct {
	Error error
}

type StatusMsg struct {
	Text    string
	IsError bool
}
