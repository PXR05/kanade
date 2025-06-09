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
	sampleOffset   int
	volumeLevel    float64
}

func NewPlayer() *Player {
	return &Player{}
}

func (p *Player) Load(filePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer != nil {
		p.streamer.Close()
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format
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

	if !p.isInitialized || p.format.SampleRate != format.SampleRate {
		bufferSize := format.SampleRate.N(time.Second / 20)
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

	p.streamer = streamer
	p.format = format
	p.ctrl = &beep.Ctrl{Streamer: streamer}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2}

	if p.volumeLevel > 0 {
		p.setVolumeUnsafe(p.volumeLevel)
	}

	p.currentFile = filePath
	p.isPlaying = false
	p.pausedPosition = 0
	p.sampleOffset = 0
	totalSamples := streamer.Len()
	p.totalLength = format.SampleRate.D(totalSamples)

	return nil
}

func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	if p.isPlaying {
		return nil
	}

	speaker.Clear()
	if p.pausedPosition > 0 {
		samplePos := p.format.SampleRate.N(p.pausedPosition)
		err := p.streamer.Seek(samplePos)
		if err != nil {
			return fmt.Errorf("failed to seek to position: %w", err)
		}
		p.sampleOffset = samplePos
	}

	p.ctrl = &beep.Ctrl{Streamer: p.streamer}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2}

	if p.volumeLevel > 0 {
		p.setVolumeUnsafe(p.volumeLevel)
	}
	speaker.Play(p.volume)
	p.ctrl.Paused = false
	p.isPlaying = true
	p.startTime = time.Now()

	return nil
}

func (p *Player) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl == nil || !p.isPlaying {
		return fmt.Errorf("not currently playing")
	}

	p.pausedPosition = p.getCurrentPositionUnsafe()
	speaker.Clear()
	p.isPlaying = false

	return nil
}

func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	speaker.Clear()
	p.isPlaying = false
	p.pausedPosition = 0
	p.sampleOffset = 0
	err := p.streamer.Seek(0)
	if err != nil {
		return fmt.Errorf("failed to reset position: %w", err)
	}

	return nil
}

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

func (p *Player) GetPlaybackPosition() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.getCurrentPositionUnsafe()
}

func (p *Player) getCurrentPositionUnsafe() time.Duration {
	if !p.isPlaying {
		return p.pausedPosition
	}

	elapsed := time.Since(p.startTime)
	currentPos := p.pausedPosition + elapsed
	if currentPos > p.totalLength {
		currentPos = p.totalLength
	}

	return currentPos
}

func (p *Player) getCurrentPosition() time.Duration {
	return p.getCurrentPositionUnsafe()
}

func (p *Player) IsPlaying() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.isPlaying
}

func (p *Player) GetTotalLength() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.totalLength
}

func (p *Player) GetCurrentFile() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.currentFile
}

func (p *Player) SetVolume(volume float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.setVolumeUnsafe(volume)
}

func (p *Player) setVolumeUnsafe(volume float64) error {
	if volume < 0 || volume > 1 {
		return fmt.Errorf("volume must be between 0.0 and 1.0")
	}

	p.volumeLevel = volume

	if p.volume == nil {
		return nil
	}
	if volume == 0 {
		p.volume.Silent = true
	} else {
		p.volume.Silent = false
		p.volume.Volume = volume - 1
	}

	return nil
}

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
