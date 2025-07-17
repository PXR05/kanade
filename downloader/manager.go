package downloader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	lib "gmp/library"
	"gmp/metadata"

	"github.com/dhowden/tag"
)

type Status int

const (
	Pending Status = iota
	InProgress
	Completed
	Failed
	Cancelled
)

type Item struct {
	ID          string
	URL         string
	Title       string
	Filename    string
	Progress    float64
	Status      Status
	ErrorMsg    string
	StartTime   time.Time
	CompletedAt time.Time
	Size        int64
	Downloaded  int64
}

type ProgressUpdate struct {
	ID         string
	Progress   float64
	Downloaded int64
	Status     Status
	ErrorMsg   string
}

type CompletionEvent struct {
	ID       string
	Item     *Item
	Song     *lib.Song
	FilePath string
	Error    error
}

type Manager struct {
	items        map[string]*Item
	queue        chan *Item
	progressChan chan ProgressUpdate
	completeChan chan CompletionEvent
	library      *lib.Library
	downloadDir  string
	workers      int
	maxRetries   int
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewManager(library *lib.Library, downloadDir string, workers int) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		items:        make(map[string]*Item),
		queue:        make(chan *Item, 100),
		progressChan: make(chan ProgressUpdate, 100),
		completeChan: make(chan CompletionEvent, 100),
		library:      library,
		downloadDir:  downloadDir,
		workers:      workers,
		maxRetries:   3,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (m *Manager) Start() {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}
}

func (m *Manager) Stop() {
	m.cancel()
	close(m.queue)
	m.wg.Wait()
	close(m.progressChan)
	close(m.completeChan)
}

func (m *Manager) AddDownload(url string) (string, error) {
	url = strings.TrimSpace(url)

	if !isValidURL(url) {
		return "", fmt.Errorf("invalid YouTube URL: %s", url)
	}

	id := generateID()
	videoID := extractVideoID(url)

	item := &Item{
		ID:        id,
		URL:       url,
		Title:     fmt.Sprintf("YouTube Video [%s]", videoID),
		Filename:  extractFilename(url),
		Progress:  0.0,
		Status:    Pending,
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.items[id] = item
	m.mu.Unlock()

	select {
	case m.queue <- item:
		return id, nil
	case <-m.ctx.Done():
		return "", fmt.Errorf("downloader is shutting down")
	}
}

func (m *Manager) RemoveDownload(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[id]
	if !exists {
		return fmt.Errorf("download not found: %s", id)
	}

	if item.Status == InProgress {
		item.Status = Cancelled
	}

	delete(m.items, id)
	return nil
}

func (m *Manager) GetDownloads() []*Item {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*Item, 0, len(m.items))
	for _, item := range m.items {
		itemCopy := *item
		items = append(items, &itemCopy)
	}
	return items
}

func (m *Manager) GetProgressChannel() <-chan ProgressUpdate {
	return m.progressChan
}

func (m *Manager) GetCompletionChannel() <-chan CompletionEvent {
	return m.completeChan
}

func (m *Manager) RetryDownload(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[id]
	if !exists {
		return fmt.Errorf("download not found: %s", id)
	}

	if item.Status != Failed {
		return fmt.Errorf("can only retry failed downloads")
	}

	item.Status = Pending
	item.Progress = 0.0
	item.Downloaded = 0
	item.ErrorMsg = ""
	item.StartTime = time.Now()

	select {
	case m.queue <- item:
		return nil
	case <-m.ctx.Done():
		return fmt.Errorf("downloader is shutting down")
	}
}

func (m *Manager) worker(workerID int) {
	defer m.wg.Done()

	for {
		select {
		case item := <-m.queue:
			if item == nil {
				return
			}
			m.processDownload(item, workerID)
		case <-m.ctx.Done():
			return
		}
	}
}

// workerID for later
func (m *Manager) processDownload(item *Item, workerID int) {
	m.updateStatus(item.ID, InProgress, "")

	err := os.MkdirAll(m.downloadDir, 0755)
	if err != nil {
		m.completeWithError(item, fmt.Errorf("failed to create download directory: %w", err))
		return
	}

	filePath, err := m.downloadYouTubeVideo(item)
	if err != nil {
		m.completeWithError(item, err)
		return
	}

	song, err := m.extractMetadataAndCreateSong(filePath)
	if err != nil {
		m.completeWithError(item, fmt.Errorf("failed to extract metadata: %w", err))
		return
	}

	m.library.AddSong(*song)

	item.CompletedAt = time.Now()
	m.updateStatus(item.ID, Completed, "")

	select {
	case m.completeChan <- CompletionEvent{
		ID:       item.ID,
		Item:     item,
		Song:     song,
		FilePath: filePath,
		Error:    nil,
	}:
	case <-m.ctx.Done():
	}
}

func (m *Manager) downloadYouTubeVideo(item *Item) (string, error) {

	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("yt-dlp not found. Please install yt-dlp: pip install yt-dlp")
	}

	cmd := exec.CommandContext(m.ctx, "yt-dlp",
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--embed-metadata",
		"--embed-thumbnail",
		"-o", "\"%(title)s.%(ext)s\"",
		"--progress",
		"--newline",
		"\""+item.URL+"\"",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	go m.monitorYtDlpProgress(item, stdout)

	var errorOutput strings.Builder
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			errorOutput.WriteString(scanner.Text() + "\n")
		}
	}()

	if err := cmd.Wait(); err != nil {
		errMsg := strings.TrimSpace(errorOutput.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("yt-dlp failed: %s", errMsg)
	}

	return m.findDownloadedFile(item)
}

func (m *Manager) monitorYtDlpProgress(item *Item, stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	progressRegex := regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)

	for scanner.Scan() {
		line := scanner.Text()

		matches := progressRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			if progress, err := strconv.ParseFloat(matches[1], 64); err == nil {
				item.Progress = progress / 100.0

				select {
				case m.progressChan <- ProgressUpdate{
					ID:       item.ID,
					Progress: item.Progress,
					Status:   InProgress,
				}:
				case <-m.ctx.Done():
					return
				}
			}
		}

		if strings.Contains(line, "[info]") && strings.Contains(line, "Downloading video") {

			if titleMatch := regexp.MustCompile(`\[info\] (.+): Downloading`).FindStringSubmatch(line); len(titleMatch) > 1 {
				item.Title = sanitizeFilename(titleMatch[1])
			}
		}
	}
}

func (m *Manager) findDownloadedFile(item *Item) (string, error) {

	entries, err := os.ReadDir(m.downloadDir)
	if err != nil {
		return "", fmt.Errorf("failed to read download directory: %w", err)
	}

	videoID := extractVideoID(item.URL)

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if info, err := entry.Info(); err == nil {
			if time.Since(info.ModTime()) < 2*time.Minute {

				ext := strings.ToLower(filepath.Ext(name))
				if ext == ".mp3" || ext == ".m4a" || ext == ".wav" || ext == ".flac" {
					candidates = append(candidates, filepath.Join(m.downloadDir, name))
				}
			}
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no downloaded file found for video ID: %s", videoID)
	}

	var newestFile string
	var newestTime time.Time

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil {
			if newestFile == "" || info.ModTime().After(newestTime) {
				newestFile = candidate
				newestTime = info.ModTime()
			}
		}
	}

	if newestFile == "" {
		return "", fmt.Errorf("could not determine downloaded file for video ID: %s", videoID)
	}

	item.Filename = filepath.Base(newestFile)

	return newestFile, nil
}

func (m *Manager) extractMetadataAndCreateSong(filePath string) (*lib.Song, error) {
	meta, err := metadata.ExtractMetadata(filePath)
	if err != nil {

		baseName := filepath.Base(filePath)
		name := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		return &lib.Song{
			Title:  name,
			Artist: "YouTube",
			Album:  "Downloaded",
			Genre:  "Unknown",
			Path:   filePath,
		}, nil
	}

	title := meta.Title()
	if title == "" {
		baseName := filepath.Base(filePath)
		title = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	}

	artist := meta.Artist()
	if artist == "" {
		artist = "YouTube"
	}

	album := meta.Album()
	if album == "" {
		album = "Downloaded"
	}

	genre := meta.Genre()
	if genre == "" {
		genre = "YouTube"
	}

	var picture *tag.Picture
	if pic := meta.Picture(); pic != nil {
		picture = pic
	}

	return &lib.Song{
		Title:   title,
		Artist:  artist,
		Album:   album,
		Genre:   genre,
		Picture: picture,
		Path:    filePath,
	}, nil
}

func (m *Manager) updateStatus(id string, status Status, errorMsg string) {
	m.mu.Lock()
	item, exists := m.items[id]
	if exists {
		item.Status = status
		if errorMsg != "" {
			item.ErrorMsg = errorMsg
		}
	}
	m.mu.Unlock()

	if exists {
		select {
		case m.progressChan <- ProgressUpdate{
			ID:         id,
			Progress:   item.Progress,
			Downloaded: item.Downloaded,
			Status:     status,
			ErrorMsg:   errorMsg,
		}:
		case <-m.ctx.Done():
		}
	}
}

func (m *Manager) completeWithError(item *Item, err error) {
	m.updateStatus(item.ID, Failed, err.Error())

	select {
	case m.completeChan <- CompletionEvent{
		ID:    item.ID,
		Item:  item,
		Error: err,
	}:
	case <-m.ctx.Done():
	}
}
