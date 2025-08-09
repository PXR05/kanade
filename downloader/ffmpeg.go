package downloader

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ffbinariesResponse struct {
	Bin map[string]struct {
		FFmpeg  string `json:"ffmpeg"`
		FFprobe string `json:"ffprobe"`
	} `json:"bin"`
}

func (m *DownloadManager) ensureFFmpeg(ctx context.Context, item *DownloadItem) (string, error) {
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}

	installDir, err := m.ffmpegInstallDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create ffmpeg dir: %w", err)
	}

	exeName := "ffmpeg"
	if runtime.GOOS == "windows" {
		exeName = "ffmpeg.exe"
	}
	localPath := filepath.Join(installDir, exeName)
	if st, err := os.Stat(localPath); err == nil && st.Size() > 0 {
		return localPath, nil
	}

	url, size, derr := m.ffbinariesURL()
	if derr != nil {
		return "", derr
	}

	zipPath := filepath.Join(installDir, "ffmpeg.zip")
	if err := m.downloadFileWithProgress(ctx, item, url, zipPath, size); err != nil {
		return "", err
	}

	ffmpegPath, err := m.extractFFmpegFromZip(zipPath, installDir)
	_ = os.Remove(zipPath)
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(ffmpegPath, 0o755)
	}
	return ffmpegPath, nil
}

func (m *DownloadManager) ffmpegInstallDir() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil || cacheRoot == "" {
		exeDir, derr := os.Executable()
		if derr == nil {
			return filepath.Join(filepath.Dir(exeDir), "ffmpeg"), nil
		}
		return filepath.Join(m.downloadDir, "ffmpeg"), nil
	}
	return filepath.Join(cacheRoot, "kanade", "ffmpeg"), nil
}

func (m *DownloadManager) ffbinariesURL() (string, int64, error) {
	platform := mapFfbinariesPlatform(runtime.GOOS, runtime.GOARCH)
	if platform == "" {
		return "", 0, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, "https://ffbinaries.com/api/v1/version/latest", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to query ffbinaries: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("ffbinaries status: %d", resp.StatusCode)
	}
	var data ffbinariesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", 0, fmt.Errorf("failed to decode ffbinaries json: %w", err)
	}
	entry, ok := data.Bin[platform]
	if !ok || entry.FFmpeg == "" {
		return "", 0, fmt.Errorf("no ffmpeg for platform %s", platform)
	}
	return entry.FFmpeg, 0, nil
}

func mapFfbinariesPlatform(goos, goarch string) string {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64", "arm64":
			return "windows-64"
		case "386":
			return "windows-32"
		}
	case "darwin":
		return "osx-64"
	case "linux":
		switch goarch {
		case "amd64":
			return "linux-64"
		case "386":
			return "linux-32"
		case "arm64":
			return "linux-arm64"
		case "arm":
			return "linux-armhf"
		}
	}
	return ""
}

func (m *DownloadManager) downloadFileWithProgress(ctx context.Context, item *DownloadItem, url, dstPath string, expectedSize int64) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download ffmpeg: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download ffmpeg: status %d", resp.StatusCode)
	}

	size := expectedSize
	if size == 0 {
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			if parsed, perr := parseInt64(cl); perr == nil {
				size = parsed
			}
		}
	}

	tmpPath := dstPath + ".part"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = out.Close() }()

	m.mu.Lock()
	item.Size = size
	item.Downloaded = 0
	item.Progress = 0
	m.mu.Unlock()

	if err := m.copyWithProgress(ctx, item, resp.Body, out); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, dstPath)
}

func (m *DownloadManager) extractFFmpegFromZip(zipPath, installDir string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open ffmpeg zip: %w", err)
	}
	defer zr.Close()

	var desiredName string
	if runtime.GOOS == "windows" {
		desiredName = "ffmpeg.exe"
	} else {
		desiredName = "ffmpeg"
	}

	var found bool
	targetPath := filepath.Join(installDir, desiredName)
	for _, f := range zr.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			continue
		}
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, "/"+desiredName) || strings.HasSuffix(lower, desiredName) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			out, err := os.Create(targetPath)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, rc); err != nil {
				_ = out.Close()
				_ = os.Remove(targetPath)
				return "", err
			}
			if err := out.Close(); err != nil {
				_ = os.Remove(targetPath)
				return "", err
			}
			found = true
			break
		}
	}
	if !found {
		return "", errors.New("ffmpeg executable not found in archive")
	}
	return targetPath, nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(s, &n)
	return n, err
}
