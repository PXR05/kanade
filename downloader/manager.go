package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lib "gmp/library"
	"gmp/metadata"

	"github.com/dhowden/tag"
	"github.com/kkdai/youtube/v2"
)

type Status int

const (
	Pending Status = iota
	InProgress
	Completed
	Failed
	Cancelled
)

type DownloadItem struct {
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
	Item     *DownloadItem
	Song     *lib.Song
	FilePath string
	Error    error
}

type DownloadManager struct {
	items        map[string]*DownloadItem
	order        []string
	queue        chan *DownloadItem
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
	cancelMap    map[string]context.CancelFunc
	cancelMapMu  sync.RWMutex
}

func NewManager(library *lib.Library, downloadDir string, workers int) *DownloadManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &DownloadManager{
		items:        make(map[string]*DownloadItem),
		order:        make([]string, 0),
		queue:        make(chan *DownloadItem, 100),
		progressChan: make(chan ProgressUpdate, 100),
		completeChan: make(chan CompletionEvent, 100),
		library:      library,
		downloadDir:  downloadDir,
		workers:      workers,
		maxRetries:   3,
		ctx:          ctx,
		cancel:       cancel,
		cancelMap:    make(map[string]context.CancelFunc),
	}
}

func (m *DownloadManager) Start() {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}
}

func (m *DownloadManager) Stop() {
	m.cancel()
	close(m.queue)
	m.wg.Wait()
	close(m.progressChan)
	close(m.completeChan)
}

func (m *DownloadManager) AddDownload(url string) (string, error) {
	url = strings.TrimSpace(url)

	if !isValidURL(url) {
		return "", fmt.Errorf("invalid YouTube URL: %s", url)
	}

	id := fmt.Sprintf("yt_%d", time.Now().UnixNano())
	videoID := extractVideoID(url)

	item := &DownloadItem{
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
	m.order = append(m.order, id)
	m.mu.Unlock()

	select {
	case m.queue <- item:
		return id, nil
	case <-m.ctx.Done():
		return "", fmt.Errorf("downloader is shutting down")
	}
}

func (m *DownloadManager) RemoveDownload(id string) error {
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
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return nil
}

func (m *DownloadManager) GetDownloads() []*DownloadItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*DownloadItem, 0, len(m.items))
	for _, id := range m.order {
		item := m.items[id]
		itemCopy := *item
		items = append(items, &itemCopy)
	}
	return items
}

func (m *DownloadManager) GetProgressChannel() <-chan ProgressUpdate {
	return m.progressChan
}

func (m *DownloadManager) GetCompletionChannel() <-chan CompletionEvent {
	return m.completeChan
}

func (m *DownloadManager) CancelDownload(id string) error {
	m.cancelMapMu.RLock()
	cancel, exists := m.cancelMap[id]
	m.cancelMapMu.RUnlock()

	if exists {
		cancel()
	}

	m.mu.Lock()
	item, itemExists := m.items[id]
	if itemExists && (item.Status == Pending || item.Status == InProgress) {
		item.Status = Cancelled
	}
	m.mu.Unlock()

	if !itemExists {
		return fmt.Errorf("download not found: %s", id)
	}

	return nil
}

func (m *DownloadManager) RetryDownload(id string) error {
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

func (m *DownloadManager) worker() {
	defer m.wg.Done()

	for {
		select {
		case item := <-m.queue:
			if item == nil {
				return
			}
			m.processDownload(item)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *DownloadManager) processDownload(item *DownloadItem) {
	downloadCtx, downloadCancel := context.WithCancel(m.ctx)
	m.cancelMapMu.Lock()
	m.cancelMap[item.ID] = downloadCancel
	m.cancelMapMu.Unlock()

	defer func() {
		m.cancelMapMu.Lock()
		delete(m.cancelMap, item.ID)
		m.cancelMapMu.Unlock()
	}()

	m.mu.RLock()
	if item.Status == Cancelled {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	m.updateStatus(item.ID, InProgress, "")

	select {
	case <-downloadCtx.Done():
		m.updateStatus(item.ID, Cancelled, "")
		return
	default:
	}

	err := os.MkdirAll(m.downloadDir, 0755)
	if err != nil {
		m.completeWithError(item, fmt.Errorf("failed to create download directory: %w", err))
		return
	}

	filePath, err := m.downloadWithRetry(downloadCtx, item)
	if err != nil {
		select {
		case <-downloadCtx.Done():
			m.updateStatus(item.ID, Cancelled, "")
			return
		default:
			m.completeWithError(item, err)
			return
		}
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

func (m *DownloadManager) downloadWithRetry(ctx context.Context, item *DownloadItem) (string, error) {
	maxRetries := 3
	baseDelay := time.Second

	for attempt := range maxRetries {
		filePath, err := m.downloadVideo(ctx, item)
		if err == nil {
			return filePath, nil
		}

		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			delay := baseDelay * time.Duration(1<<attempt)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		return "", err
	}

	return "", fmt.Errorf("max retries exceeded")
}

func (m *DownloadManager) downloadVideo(ctx context.Context, item *DownloadItem) (string, error) {
	select {
	case <-m.ctx.Done():
		return "", fmt.Errorf("download cancelled")
	default:
	}

	client, err := m.createClient()
	if err != nil {
		return "", fmt.Errorf("failed to create client: %w", err)
	}

	video, err := client.GetVideo(item.URL)
	if err != nil {
		fallbackClient := youtube.Client{}
		fallbackVideo, fallbackErr := fallbackClient.GetVideo(item.URL)
		if fallbackErr != nil {
			return "", fmt.Errorf("failed to get video info for URL %s (tried both enhanced and fallback clients): original error: %w, fallback error: %v", item.URL, err, fallbackErr)
		}
		client = fallbackClient
		video = fallbackVideo
	}

	item.Title = sanitizeFilename(video.Title)

	audioFormats := video.Formats.WithAudioChannels().Type("audio")
	if len(audioFormats) == 0 {
		audioFormats = video.Formats.WithAudioChannels()
	}

	if len(audioFormats) == 0 {
		return "", fmt.Errorf("no audio formats available")
	}

	var bestFormat *youtube.Format
	var bestBitrate int

	for i := range audioFormats {
		format := &audioFormats[i]

		if format.AudioChannels == 0 {
			continue
		}

		var bitrate int
		if format.Bitrate > 0 {
			bitrate = format.Bitrate
		} else {
			switch format.AudioSampleRate {
			case "48000":
				bitrate = 160
			case "44100":
				bitrate = 128
			default:
				bitrate = 96
			}
		}

		qualityScore := bitrate * 100

		if isMimeTypeAudioOnly(format.MimeType) {
			qualityScore += 50
		}

		switch format.AudioSampleRate {
		case "48000":
			qualityScore += 20
		case "44100":
			qualityScore += 10
		}

		if format.AudioChannels >= 2 {
			qualityScore += 5
		}

		var bestQuality int
		if bestFormat != nil {
			bestQuality = bestBitrate * 100
			if isMimeTypeAudioOnly(bestFormat.MimeType) {
				bestQuality += 50
			}
			switch bestFormat.AudioSampleRate {
			case "48000":
				bestQuality += 20
			case "44100":
				bestQuality += 10
			}
			if bestFormat.AudioChannels >= 2 {
				bestQuality += 5
			}
		}

		if bestFormat == nil || qualityScore > bestQuality {
			bestFormat = format
			bestBitrate = bitrate
		}
	}

	if bestFormat == nil {
		bestFormat = &audioFormats[0]
	}

	tempExt := getExtensionFromMimeType(bestFormat.MimeType)
	tempFilename := fmt.Sprintf("%s_temp%s", item.Title, tempExt)
	tempFilePath := filepath.Join(m.downloadDir, tempFilename)

	finalFilename := fmt.Sprintf("%s.mp3", item.Title)
	finalFilePath := filepath.Join(m.downloadDir, finalFilename)

	stream, size, err := client.GetStream(video, bestFormat)
	if err != nil {
		return "", fmt.Errorf("failed to get stream: %w", err)
	}
	defer stream.Close()

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	item.Size = size

	err = m.copyWithProgress(ctx, item, stream, tempFile)
	if err != nil {
		os.Remove(tempFilePath)
		return "", err
	}
	tempFile.Close()

	thumbnailPath, err := m.downloadThumbnail(video, item.Title)
	if err != nil {
		thumbnailPath = ""
	}

	m.updateStatus(item.ID, InProgress, "Converting to MP3...")

	err = m.convertToMP3WithMetadata(ctx, tempFilePath, finalFilePath, thumbnailPath, video)
	if err != nil {
		copyErr := m.copyFileAsMP3(tempFilePath, finalFilePath)
		if copyErr != nil {
			os.Remove(tempFilePath)
			if thumbnailPath != "" {
				os.Remove(thumbnailPath)
			}
			return "", fmt.Errorf("failed to convert to MP3 and fallback copy failed: convert error: %w, copy error: %v", err, copyErr)
		}
	}

	os.Remove(tempFilePath)
	if thumbnailPath != "" {
		os.Remove(thumbnailPath)
	}

	item.Filename = finalFilename
	return finalFilePath, nil
}

func (m *DownloadManager) createClient() (youtube.Client, error) {
	client := youtube.Client{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSHandshakeTimeout: 10 * time.Second,
				DisableCompression:  false,
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
			},
			Timeout: 60 * time.Second,
		},
	}

	client.HTTPClient.Transport = &headerTransport{
		Transport: client.HTTPClient.Transport,
		Headers: map[string]string{
			"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"Accept-Language":           "en-US,en;q=0.9",
			"Accept-Encoding":           "gzip, deflate, br",
			"Cache-Control":             "no-cache",
			"Pragma":                    "no-cache",
			"Sec-Ch-Ua":                 `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
			"Sec-Ch-Ua-Mobile":          "?0",
			"Sec-Ch-Ua-Platform":        `"Windows"`,
			"Sec-Fetch-Dest":            "document",
			"Sec-Fetch-Mode":            "navigate",
			"Sec-Fetch-Site":            "none",
			"Sec-Fetch-User":            "?1",
			"Upgrade-Insecure-Requests": "1",
			"X-Requested-With":          "XMLHttpRequest",
		},
	}

	return client, nil
}

func (m *DownloadManager) downloadThumbnail(video *youtube.Video, title string) (string, error) {
	if len(video.Thumbnails) == 0 {
		return "", fmt.Errorf("no thumbnails available")
	}

	var thumbnail youtube.Thumbnail
	var bestArea uint = 0
	for _, t := range video.Thumbnails {
		area := t.Width * t.Height
		if area > bestArea {
			bestArea = area
			thumbnail = t
		}
	}

	if bestArea == 0 {
		thumbnail = video.Thumbnails[len(video.Thumbnails)-1]
	}

	resp, err := http.Get(thumbnail.URL)
	if err != nil {
		return "", fmt.Errorf("failed to download thumbnail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download thumbnail: status %d", resp.StatusCode)
	}

	ext := ".jpg"
	contentType := resp.Header.Get("Content-Type")
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	default:
		if strings.Contains(thumbnail.URL, ".png") {
			ext = ".png"
		} else if strings.Contains(thumbnail.URL, ".webp") {
			ext = ".webp"
		}
	}

	thumbnailPath := filepath.Join(m.downloadDir, fmt.Sprintf("%s_thumb%s", sanitizeFilename(title), ext))
	file, err := os.Create(thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(thumbnailPath)
		return "", fmt.Errorf("failed to save thumbnail: %w", err)
	}

	return thumbnailPath, nil
}

func (m *DownloadManager) convertToMP3WithMetadata(ctx context.Context, inputPath, outputPath, thumbnailPath string, video *youtube.Video) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	args := []string{"-y"}
	args = append(args, "-i", inputPath)
	args = append(args, "-c:a", "libmp3lame")
	args = append(args, "-b:a", "192k")
	args = append(args, "-ar", "44100")
	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w, stderr: %s", err, stderr.String())
	}

	err = m.embedMetadataWithWriter(outputPath, thumbnailPath, video)
	if err != nil {
		return fmt.Errorf("failed to embed metadata: %w", err)
	}

	return nil
}

func (m *DownloadManager) embedMetadataWithWriter(filePath, thumbnailPath string, video *youtube.Video) error {
	meta := metadata.Metadata{
		Title:  video.Title,
		Artist: video.Author,
		Album:  video.Title,
		Year:   video.PublishDate.Format("2006"),
		Genre:  "Unknown",
	}

	if thumbnailPath != "" {
		thumbnailData, err := os.ReadFile(thumbnailPath)
		if err == nil {
			ext := strings.ToLower(filepath.Ext(thumbnailPath))
			var mimeType string
			switch ext {
			case ".jpg", ".jpeg":
				mimeType = "image/jpeg"
			case ".png":
				mimeType = "image/png"
			case ".gif":
				mimeType = "image/gif"
			case ".webp":
				mimeType = "image/webp"
			default:
				mimeType = "image/jpeg"
			}
			meta.AlbumArt = metadata.Picture{
				Ext:         ext,
				MIMEType:    mimeType,
				Type:        "Front cover",
				Description: "Front cover",
				Data:        thumbnailData,
			}
		}
	}

	return metadata.WriteMetadata(filePath, meta)
}

func (m *DownloadManager) copyFileAsMP3(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func (m *DownloadManager) extractMetadataAndCreateSong(filePath string) (*lib.Song, error) {
	meta, err := metadata.ExtractMetadata(filePath)
	if err != nil {

		baseName := filepath.Base(filePath)
		name := strings.TrimSuffix(baseName, filepath.Ext(baseName))

		return &lib.Song{
			Title:  name,
			Artist: "Unknown",
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
		artist = "Unknown"
	}

	album := meta.Album()
	if album == "" {
		album = title
	}

	genre := meta.Genre()
	if genre == "" {
		genre = "Unknown"
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

func (m *DownloadManager) updateStatus(id string, status Status, errorMsg string) {
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

func (m *DownloadManager) completeWithError(item *DownloadItem, err error) {
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

func (m *DownloadManager) copyWithProgress(ctx context.Context, item *DownloadItem, src io.Reader, dst io.Writer) error {
	buffer := make([]byte, 32*1024)
	var written int64

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("download cancelled")
		default:
		}

		n_read, err_read := src.Read(buffer)
		if n_read > 0 {
			n_write, err_write := dst.Write(buffer[0:n_read])
			if n_write < 0 || n_read < n_write {
				n_write = 0
				if err_write == nil {
					err_write = fmt.Errorf("invalid write result")
				}
			}
			written += int64(n_write)
			if err_write != nil {
				return err_write
			}
			if n_read != n_write {
				return fmt.Errorf("short write")
			}

			item.Downloaded = written
			if item.Size > 0 {
				item.Progress = float64(written) / float64(item.Size)
			}

			select {
			case m.progressChan <- ProgressUpdate{
				ID:         item.ID,
				Progress:   item.Progress,
				Downloaded: written,
				Status:     InProgress,
			}:
			case <-ctx.Done():
				return fmt.Errorf("download cancelled")
			}
		}
		if err_read != nil {
			if err_read != io.EOF {
				return err_read
			}
			break
		}
	}
	return nil
}

type headerTransport struct {
	Transport http.RoundTripper
	Headers   map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.Headers {
		req.Header.Set(key, value)
	}

	if strings.Contains(req.URL.String(), "youtube.com") || strings.Contains(req.URL.String(), "googlevideo.com") {
		req.Header.Set("Referer", "https://www.youtube.com/")
	}

	if t.Transport == nil {
		t.Transport = http.DefaultTransport
	}

	return t.Transport.RoundTrip(req)
}
