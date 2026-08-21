package audio

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
)

const externalSampleRate = 44100

var errNoNativeDecoder = errors.New("no native decoder")

func isExternalStream(s beep.StreamSeekCloser) bool {
	_, ok := s.(*externalStream)
	return ok
}

type externalStream struct {
	exe  string
	path string

	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	stdout io.ReadCloser
	stderr *strings.Builder
	pos    int
	length int
	buf    []byte
	err    error
}

func decodeExternal(exe, path string) (beep.StreamSeekCloser, beep.Format, error) {
	length := 0
	if d, err := cachedDuration(exe, path); err == nil && d > 0 {
		length = beep.SampleRate(externalSampleRate).N(d)
	}

	s := &externalStream{
		exe:    exe,
		path:   path,
		length: length,
	}

	if err := s.start(0); err != nil {
		return nil, beep.Format{}, err
	}

	return s, beep.Format{
		SampleRate:  beep.SampleRate(externalSampleRate),
		NumChannels: 2,
		Precision:   2,
	}, nil
}

func probeDuration(exe, path string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe,
		"-hide_banner", "-nostdin", "-i", path)
	output, _ := cmd.CombinedOutput()

	for _, line := range strings.Split(string(output), "\n") {
		idx := strings.Index(line, "Duration:")
		if idx < 0 {
			continue
		}
		fields := strings.Fields(line[idx:])
		if len(fields) < 2 {
			break
		}
		parts := strings.Split(fields[1], ":")
		if len(parts) != 3 {
			break
		}
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		var s float64
		fmt.Sscanf(parts[2], "%f", &s)
		return time.Duration(h)*time.Hour +
			time.Duration(m)*time.Minute +
			time.Duration(s*float64(time.Second)), nil
	}
	return 0, fmt.Errorf("could not determine duration")
}

var durationCache sync.Map

func cachedDuration(exe, path string) (time.Duration, error) {
	if d, ok := durationCache.Load(path); ok {
		return d.(time.Duration), nil
	}
	d, err := probeDuration(exe, path)
	if err != nil {
		return 0, err
	}
	durationCache.Store(path, d)
	return d, nil
}

func (s *externalStream) start(pos int) error {
	ctx, cancel := context.WithCancel(context.Background())
	args := []string{"-v", "error", "-nostdin"}
	if pos > 0 {
		seconds := float64(pos) / float64(externalSampleRate)
		args = append(args, "-ss", strconv.FormatFloat(seconds, 'f', 3, 64))
	}
	args = append(args,
		"-i", s.path,
		"-vn",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-ar", strconv.Itoa(externalSampleRate),
		"-ac", "2",
		"pipe:1")

	cmd := exec.CommandContext(ctx, s.exe, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("ffmpeg pipe failed: %w", err)
	}
	stderrBuf := &strings.Builder{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	s.cmd = cmd
	s.cancel = cancel
	s.stdout = stdout
	s.stderr = stderrBuf
	return nil
}

func (s *externalStream) stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.stdout != nil {
		s.stdout.Close()
		s.stdout = nil
	}
	if s.cmd != nil {
		s.cmd.Wait()
		s.cmd = nil
	}
	s.cancel = nil
}

func (s *externalStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stop()
	return nil
}

func (s *externalStream) Len() int {
	return s.length
}

func (s *externalStream) Position() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pos
}

func (s *externalStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *externalStream) Seek(p int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p < 0 || (s.length > 0 && p > s.length) {
		return fmt.Errorf("seek position out of bounds: %d (max: %d)", p, s.length)
	}

	s.stop()
	if err := s.start(p); err != nil {
		return err
	}
	s.pos = p
	return nil
}

func (s *externalStream) Stream(samples [][2]float64) (n int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return 0, false
	}

	frames := len(samples)
	need := frames * 4
	if cap(s.buf) < need {
		s.buf = make([]byte, need)
	}
	buf := s.buf[:need]

	read := 0
	var readErr error
	for read < need {
		rn, err := s.stdout.Read(buf[read:])
		read += rn
		if err != nil {
			readErr = err
			break
		}
	}

	framesRead := read / 4
	for i := range framesRead {
		l := int16(binary.LittleEndian.Uint16(buf[i*4:]))
		r := int16(binary.LittleEndian.Uint16(buf[i*4+2:]))
		samples[i][0] = float64(l) / 32768.0
		samples[i][1] = float64(r) / 32768.0
	}
	s.pos += framesRead

	if readErr != nil {
		s.setErr(readErr)
		return framesRead, false
	}
	return framesRead, true
}

func (s *externalStream) setErr(err error) {
	if s.err == nil && err != nil && err != io.EOF {
		msg := strings.TrimSpace(s.stderr.String())
		if msg != "" {
			s.err = fmt.Errorf("%w: %s", err, msg)
		} else {
			s.err = err
		}
	}
}
