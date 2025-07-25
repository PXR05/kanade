package audio

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
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
	speakerFormat  beep.Format
	isInitialized  bool
	isPlaying      bool
	currentFile    string
	totalLength    time.Duration
	startTime      time.Time
	pausedPosition time.Duration
	sampleOffset   int
	volumeLevel    float64

	loadingMu      sync.Mutex
	switchingTrack int32
	isClosed       int32
	lastError      error
	errorCallback  func(error)

	playbackDone chan struct{}
	playbackMu   sync.Mutex

	lastLoadTime time.Time
	loadThrottle time.Duration

	trackLoadCount int
	lastDeepClean  time.Time

	lastPositionUpdate time.Time
	cachedPosition     time.Duration
}

func NewPlayer() *Player {
	return &Player{
		volumeLevel:  0.5,
		playbackDone: make(chan struct{}, 1),
		loadThrottle: 50 * time.Millisecond,
	}
}

func (p *Player) SetErrorCallback(callback func(error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errorCallback = callback
}

func (p *Player) reportError(err error) {
	p.mu.Lock()
	p.lastError = err
	callback := p.errorCallback
	p.mu.Unlock()

	if callback != nil {
		callback(err)
	}
}

func (p *Player) GetLastError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError
}

func (p *Player) Load(filePath string) error {
	p.loadingMu.Lock()
	defer p.loadingMu.Unlock()

	if atomic.LoadInt32(&p.isClosed) == 1 {
		return fmt.Errorf("player is closed")
	}

	timeSinceLastLoad := time.Since(p.lastLoadTime)
	if timeSinceLastLoad < p.loadThrottle {
		waitTime := p.loadThrottle - timeSinceLastLoad
		time.Sleep(waitTime)
	}
	p.lastLoadTime = time.Now()

	atomic.StoreInt32(&p.switchingTrack, 1)
	defer atomic.StoreInt32(&p.switchingTrack, 0)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.trackLoadCount++

	shouldDeepClean := p.trackLoadCount%10 == 0 || time.Since(p.lastDeepClean) > 60*time.Second
	if shouldDeepClean {

		p.mu.Unlock()
		p.DeepCleanup()
		p.mu.Lock()
	}

	if p.isPlaying {
		speaker.Clear()
		p.isPlaying = false

		time.Sleep(10 * time.Millisecond)
	}

	speaker.Clear()
	time.Sleep(2 * time.Millisecond)
	speaker.Clear()

	if p.streamer != nil {
		if err := p.streamer.Close(); err != nil {
			p.reportError(fmt.Errorf("failed to close previous streamer: %w", err))
		}
		p.streamer = nil
	}

	p.playbackMu.Lock()
	if p.playbackDone != nil {
		select {
		case <-p.playbackDone:
		default:
		}
	}
	p.playbackMu.Unlock()

	p.ctrl = nil
	p.volume = nil

	runtime.GC()

	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not accessible: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	var fileToClose *os.File = file
	defer func() {
		if fileToClose != nil {
			fileToClose.Close()
		}
	}()

	var streamer beep.StreamSeekCloser
	var format beep.Format
	ext := getFileExtension(filePath)

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
		if err != nil {
			return fmt.Errorf("failed to decode MP3: %w", err)
		}
	case ".wav":
		streamer, format, err = wav.Decode(file)
		if err != nil {
			return fmt.Errorf("failed to decode WAV: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	if streamer == nil {
		return fmt.Errorf("failed to create audio streamer")
	}

	totalSamples := streamer.Len()
	if totalSamples <= 0 {
		streamer.Close()
		return fmt.Errorf("file contains no audio data or is corrupted")
	}

	shouldReinitSpeaker := !p.isInitialized
	if p.isInitialized {
		speaker.Clear()
		time.Sleep(15 * time.Millisecond)
	}

	if shouldReinitSpeaker {
		speakerSampleRate := beep.SampleRate(44100)
		bufferSize := max(speakerSampleRate.N(time.Second/40), 256)

		err = speaker.Init(speakerSampleRate, bufferSize)
		if err != nil {
			streamer.Close()
			return fmt.Errorf("failed to initialize speaker: %w", err)
		}
		p.isInitialized = true
		p.speakerFormat = beep.Format{
			SampleRate:  speakerSampleRate,
			NumChannels: 2,
			Precision:   2,
		}
	}

	var finalStreamer beep.Streamer = streamer
	finalFormat := format

	if format.SampleRate != p.speakerFormat.SampleRate {
		quality := 1
		resampler := beep.Resample(quality, format.SampleRate, p.speakerFormat.SampleRate, streamer)

		finalStreamer = resampler
		finalFormat.SampleRate = p.speakerFormat.SampleRate

		originalLength := format.SampleRate.D(totalSamples)
		totalSamples = p.speakerFormat.SampleRate.N(originalLength)
	}

	var finalStreamSeekCloser beep.StreamSeekCloser
	if format.SampleRate != p.speakerFormat.SampleRate {
		finalStreamSeekCloser = &seekWrapper{
			Streamer:       finalStreamer,
			original:       streamer,
			length:         totalSamples,
			originalFormat: format,
			targetFormat:   finalFormat,
			quality:        1,
		}
	} else {
		finalStreamSeekCloser = streamer
	}

	p.streamer = finalStreamSeekCloser
	p.format = finalFormat
	p.currentFile = filePath
	p.isPlaying = false
	p.pausedPosition = 0
	p.sampleOffset = 0
	p.totalLength = finalFormat.SampleRate.D(totalSamples)
	p.lastError = nil

	p.ctrl = &beep.Ctrl{Streamer: finalStreamSeekCloser}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2}

	if err := p.setVolumeUnsafe(p.volumeLevel); err != nil {
		p.reportError(fmt.Errorf("failed to set volume: %w", err))
	}

	fileToClose = nil

	return nil
}

type seekWrapper struct {
	beep.Streamer
	original       beep.StreamSeekCloser
	length         int
	position       int
	originalFormat beep.Format
	targetFormat   beep.Format
	quality        int
	mu             sync.Mutex
}

func (s *seekWrapper) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.original.Close()
}

func (s *seekWrapper) Len() int {
	return s.length
}

func (s *seekWrapper) Position() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.position
}

func (s *seekWrapper) Seek(p int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p < 0 || p > s.length {
		return fmt.Errorf("seek position out of bounds: %d (max: %d)", p, s.length)
	}

	targetDuration := s.targetFormat.SampleRate.D(p)
	originalPos := s.originalFormat.SampleRate.N(targetDuration)

	err := s.original.Seek(originalPos)
	if err != nil {
		return err
	}

	s.Streamer = beep.Resample(s.quality, s.originalFormat.SampleRate, s.targetFormat.SampleRate, s.original)
	s.position = p

	return nil
}

func (s *seekWrapper) Stream(samples [][2]float64) (n int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok = s.Streamer.Stream(samples)
	s.position += n
	return n, ok
}

func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.isClosed) == 1 {
		return fmt.Errorf("player is closed")
	}

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	if p.isPlaying {
		return nil
	}

	speaker.Clear()
	time.Sleep(2 * time.Millisecond)

	if p.pausedPosition > 0 {
		samplePos := p.format.SampleRate.N(p.pausedPosition)
		if err := p.streamer.Seek(samplePos); err != nil {
			p.pausedPosition = 0
			if err := p.streamer.Seek(0); err != nil {
				return fmt.Errorf("failed to reset to beginning: %w", err)
			}
		}
		p.sampleOffset = samplePos
	}

	p.ctrl = &beep.Ctrl{Streamer: p.streamer}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2}

	if err := p.setVolumeUnsafe(p.volumeLevel); err != nil {
		return fmt.Errorf("failed to set volume: %w", err)
	}

	p.playbackMu.Lock()
	select {
	case <-p.playbackDone:
	default:
	}
	p.playbackMu.Unlock()

	completion := beep.Seq(p.volume, beep.Callback(func() {
		select {
		case p.playbackDone <- struct{}{}:
		default:
		}
	}))

	speaker.Play(completion)
	p.ctrl.Paused = false
	p.isPlaying = true
	p.startTime = time.Now()

	return nil
}

func (p *Player) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.isClosed) == 1 {
		return fmt.Errorf("player is closed")
	}

	if p.ctrl == nil || !p.isPlaying {
		return fmt.Errorf("not currently playing")
	}

	p.pausedPosition = p.getCurrentPositionUnsafe()
	speaker.Clear()
	time.Sleep(1 * time.Millisecond)
	p.isPlaying = false

	return nil
}

func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.isClosed) == 1 {
		return fmt.Errorf("player is closed")
	}

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	speaker.Clear()
	time.Sleep(1 * time.Millisecond)
	p.isPlaying = false
	p.pausedPosition = 0
	p.sampleOffset = 0

	p.playbackMu.Lock()
	select {
	case <-p.playbackDone:
	default:
	}
	p.playbackMu.Unlock()

	if err := p.streamer.Seek(0); err != nil {
		return nil
	}

	return nil
}

func (p *Player) Seek(position time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if atomic.LoadInt32(&p.isClosed) == 1 {
		return fmt.Errorf("player is closed")
	}

	if p.streamer == nil {
		return fmt.Errorf("no file loaded")
	}

	if position < 0 || position > p.totalLength {
		return fmt.Errorf("position out of bounds: %v (max: %v)", position, p.totalLength)
	}

	samplePos := p.format.SampleRate.N(position)
	if err := p.streamer.Seek(samplePos); err != nil {
		return fmt.Errorf("failed to seek to position %v: %w", position, err)
	}

	p.sampleOffset = samplePos
	p.pausedPosition = position
	if p.isPlaying {
		p.startTime = time.Now()
	}

	return nil
}

func (p *Player) IsAtEnd() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.totalLength == 0 {
		return false
	}

	currentPos := p.getCurrentPositionUnsafe()
	return currentPos >= p.totalLength-time.Millisecond*100
}

func (p *Player) HasPlaybackFinished() bool {
	select {
	case <-p.playbackDone:
		return true
	default:
		return false
	}
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

	now := time.Now()
	if now.Sub(p.lastPositionUpdate) < 50*time.Millisecond {
		return p.cachedPosition
	}

	elapsed := time.Since(p.startTime)
	currentPos := min(p.pausedPosition+elapsed, p.totalLength)

	p.lastPositionUpdate = now
	p.cachedPosition = currentPos

	return currentPos
}

func (p *Player) IsPlaying() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if atomic.LoadInt32(&p.switchingTrack) == 1 {
		return false
	}

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
		return fmt.Errorf("volume must be between 0.0 and 1.0, got: %f", volume)
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
	atomic.StoreInt32(&p.isClosed, 1)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isPlaying {
		speaker.Clear()
		time.Sleep(10 * time.Millisecond)
		p.isPlaying = false
	}

	speaker.Clear()

	var err error
	if p.streamer != nil {
		err = p.streamer.Close()
		p.streamer = nil
	}

	p.ctrl = nil
	p.volume = nil

	p.currentFile = ""
	p.totalLength = 0
	p.pausedPosition = 0
	p.sampleOffset = 0

	p.playbackMu.Lock()
	if p.playbackDone != nil {
		select {
		case <-p.playbackDone:
		default:
		}
		close(p.playbackDone)
		p.playbackDone = nil
	}
	p.playbackMu.Unlock()

	return err
}

func (p *Player) ForceGC() {
	runtime.GC()
	runtime.GC()
}

func (p *Player) DeepCleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := 0; i < 2; i++ {
		speaker.Clear()
		time.Sleep(5 * time.Millisecond)
	}

	for i := 0; i < 2; i++ {
		runtime.GC()
		time.Sleep(2 * time.Millisecond)
	}

	p.trackLoadCount = 0
	p.lastDeepClean = time.Now()
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
