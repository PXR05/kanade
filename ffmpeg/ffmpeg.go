package ffmpeg

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
	"sync"
	"time"
)

var (
	once     sync.Once
	ready    = make(chan struct{})
	exePath  string
	ensureMu sync.Mutex
)

type ffbinariesResponse struct {
	Bin map[string]struct {
		FFmpeg string `json:"ffmpeg"`
	} `json:"bin"`
}

func init() {
	EnsureAsync()
}

func EnsureAsync() {
	once.Do(func() {
		go func() {
			path, err := resolve()
			if err == nil {
				exePath = path
			}
			close(ready)
		}()
	})
}

func Wait(ctx context.Context) (string, error) {
	select {
	case <-ready:
		if exePath == "" {
			return "", errors.New("ffmpeg unavailable: install it or add it to PATH")
		}
		return exePath, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func resolve() (string, error) {
	ensureMu.Lock()
	defer ensureMu.Unlock()

	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}

	dir, err := installDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create ffmpeg dir: %w", err)
	}

	exeName := "ffmpeg"
	if runtime.GOOS == "windows" {
		exeName = "ffmpeg.exe"
	}
	local := filepath.Join(dir, exeName)
	if st, err := os.Stat(local); err == nil && st.Size() > 0 {
		return local, nil
	}

	url, err := downloadURL()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	zipPath := filepath.Join(dir, "ffmpeg.zip")
	if err := download(ctx, url, zipPath); err != nil {
		os.Remove(zipPath)
		return "", err
	}
	path, err := extract(zipPath, dir, exeName)
	_ = os.Remove(zipPath)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, 0o755)
	}
	return path, nil
}

func installDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("could not determine cache directory: %w", herr)
		}
		return filepath.Join(home, ".kanade", "ffmpeg"), nil
	}
	return filepath.Join(cache, "kanade", "ffmpeg"), nil
}

func downloadURL() (string, error) {
	platform := platformKey(runtime.GOOS, runtime.GOARCH)
	if platform == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, "https://ffbinaries.com/api/v1/version/latest", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query ffbinaries: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ffbinaries status: %d", resp.StatusCode)
	}

	var data ffbinariesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode ffbinaries json: %w", err)
	}
	entry, ok := data.Bin[platform]
	if !ok || entry.FFmpeg == "" {
		return "", fmt.Errorf("no ffmpeg build for %s", platform)
	}
	return entry.FFmpeg, nil
}

func platformKey(goos, goarch string) string {
	switch goos {
	case "windows":
		switch goarch {
		case "amd64", "arm64":
			return "windows-64"
		case "386":
			return "windows-32"
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "osx-64"
		case "arm64":
			return "osx-64"
		}
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

func download(ctx context.Context, url, dst string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download ffmpeg: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download ffmpeg: status %d", resp.StatusCode)
	}

	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("failed to download ffmpeg: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func extract(zipPath, dir, exeName string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to open ffmpeg zip: %w", err)
	}
	defer zr.Close()

	target := filepath.Join(dir, exeName)
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if strings.HasSuffix(name, "/"+exeName) || strings.HasSuffix(name, exeName) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			out, err := os.Create(target)
			if err != nil {
				rc.Close()
				return "", err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				rc.Close()
				os.Remove(target)
				return "", err
			}
			out.Close()
			rc.Close()
			return target, nil
		}
	}
	return "", errors.New("ffmpeg executable not found in archive")
}
