package audio

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// Player represents an audio player with basic playback controls
type Player struct {
	mu             sync.RWMutex
	streamer       beep.StreamSeekCloser
	ctrl           *beep.Ctrl
	volume         *effects.Volume
	format         beep.Format
	isInitialized  bool
	isPlaying      bool
	currentFile    string
	totalLength    time.Duration
	startTime      time.Time
	pausedPosition time.Duration
	sampleOffset   int     // Track sample offset for accurate position
	volumeLevel    float64 // Store volume level for persistence
}

// NewPlayer creates a new audio player instance
func NewPlayer() *Player {
	return &Player{}
}

// Load loads an audio file for playback
func (p *Player) Load(filePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close existing streamer if any
	if p.streamer != nil {
		p.streamer.Close()
	}

	// Open the audio file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	// Determine file type and decode accordingly
	var streamer beep.StreamSeekCloser
	var format beep.Format

	// Get file extension to determine decoder
	ext := getFileExtension(filePath)
	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".wav":
		streamer, format, err = wav.Decode(file)
	default:
		file.Close()
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	if err != nil {
		file.Close()
		return fmt.Errorf("failed to decode audio: %w", err)
	}

	// Initialize speaker if not already done or if format changed
	if !p.isInitialized || p.format.SampleRate != format.SampleRate {
		// Use a more conservative buffer size
		bufferSize := format.SampleRate.N(time.Second / 20) // 50ms buffer
		if bufferSize < 1024 {
			bufferSize = 1024
		}

		err = speaker.Init(format.SampleRate, bufferSize)
		if err != nil {
			streamer.Close()
			return fmt.Errorf("failed to initialize speaker: %w", err)
		}
		p.isInitialized = true
	}

	// Create control wrapper
	p.streamer = streamer
	p.format = format
	p.ctrl = &beep.Ctrl{Streamer: streamer}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2}

	// Restore volume level if it was set before
	if p.volumeLevel > 0 {
		p.setVolumeUnsafe(p.volumeLevel)
	}

	p.currentFile = filePath
	p.isPlaying = false
	p.pausedPosition = 0
	p.sampleOffset = 0

	// Calculate total length
	totalSamples := streamer.Len()
	p.totalLength = format.SampleRate.D(totalSamples)

	return nil
}

// Play starts or resumes playback
func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	if p.isPlaying {
		return nil // Already playing
	}

	// Clear any existing audio from the speaker buffer
	speaker.Clear()

	// Seek to paused position if resuming
	if p.pausedPosition > 0 {
		samplePos := p.format.SampleRate.N(p.pausedPosition)
		err := p.streamer.Seek(samplePos)
		if err != nil {
			return fmt.Errorf("failed to seek to position: %w", err)
		}
		p.sampleOffset = samplePos
	}

	// Create a fresh control wrapper to avoid buffer corruption
	p.ctrl = &beep.Ctrl{Streamer: p.streamer}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2}

	// Restore volume setting
	if p.volumeLevel > 0 {
		p.setVolumeUnsafe(p.volumeLevel)
	}

	// Start playback
	speaker.Play(p.volume)
	p.ctrl.Paused = false
	p.isPlaying = true
	p.startTime = time.Now()

	return nil
}

// Pause pauses the playback
func (p *Player) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl == nil || !p.isPlaying {
		return fmt.Errorf("not currently playing")
	}

	// Update paused position before stopping
	p.pausedPosition = p.getCurrentPositionUnsafe()

	// Stop playback and clear the buffer completely
	speaker.Clear()
	p.isPlaying = false

	return nil
}

// Stop stops the playback and resets position
func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	// Clear the speaker buffer completely
	speaker.Clear()
	p.isPlaying = false
	p.pausedPosition = 0
	p.sampleOffset = 0

	// Reset to beginning
	err := p.streamer.Seek(0)
	if err != nil {
		return fmt.Errorf("failed to reset position: %w", err)
	}

	return nil
}

// Seek seeks to a specific position in the audio
func (p *Player) Seek(position time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	if position < 0 || position > p.totalLength {
		return fmt.Errorf("position out of bounds")
	}

	samplePos := p.format.SampleRate.N(position)
	err := p.streamer.Seek(samplePos)
	if err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	p.sampleOffset = samplePos
	p.pausedPosition = position
	if p.isPlaying {
		p.startTime = time.Now()
	}

	return nil
}

// GetPlaybackPosition returns the current playback position
func (p *Player) GetPlaybackPosition() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getCurrentPositionUnsafe()
}

// getCurrentPositionUnsafe calculates current position (must be called with lock held)
func (p *Player) getCurrentPositionUnsafe() time.Duration {
	if !p.isPlaying {
		return p.pausedPosition
	}

	// Calculate position based on time elapsed since play started
	elapsed := time.Since(p.startTime)
	currentPos := p.pausedPosition + elapsed

	// Ensure we don't exceed total duration
	if currentPos > p.totalLength {
		currentPos = p.totalLength
	}

	return currentPos
}

// getCurrentPosition is the public version that requires lock
func (p *Player) getCurrentPosition() time.Duration {
	return p.getCurrentPositionUnsafe()
}

// IsPlaying returns whether the player is currently playing
func (p *Player) IsPlaying() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.isPlaying
}

// GetTotalLength returns the total length of the loaded audio
func (p *Player) GetTotalLength() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.totalLength
}

// GetCurrentFile returns the path of the currently loaded file
func (p *Player) GetCurrentFile() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.currentFile
}

// SetVolume sets the playback volume (0.0 to 1.0)
func (p *Player) SetVolume(volume float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.setVolumeUnsafe(volume)
}

// setVolumeUnsafe sets volume without locking (must be called with lock held)
func (p *Player) setVolumeUnsafe(volume float64) error {
	if volume < 0 || volume > 1 {
		return fmt.Errorf("volume must be between 0.0 and 1.0")
	}

	p.volumeLevel = volume

	if p.volume == nil {
		return nil // Volume will be set when streamer is created
	}

	// Convert to decibels
	if volume == 0 {
		p.volume.Silent = true
	} else {
		p.volume.Silent = false
		p.volume.Volume = volume - 1 // beep uses -1 to 0 range
	}

	return nil
}

// Close closes the player and releases resources
func (p *Player) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer != nil {
		err := p.streamer.Close()
		p.streamer = nil
		return err
	}

	return nil
}

// getFileExtension is a helper function to extract file extension
func getFileExtension(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '.' {
			return filePath[i:]
		}
		if filePath[i] == '/' || filePath[i] == '\\' {
			break
		}
	}
	return ""
}
