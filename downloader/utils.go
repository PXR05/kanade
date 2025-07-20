package downloader

import (
	"fmt"
	"path"
	"strings"
	"time"
)

func isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return false
	}

	return isYouTubeURL(urlStr)
}

func isYouTubeURL(urlStr string) bool {
	urlStr = strings.TrimSpace(urlStr)
	urlLower := strings.ToLower(urlStr)

	hasYouTube := strings.Contains(urlLower, "youtube.com") || strings.Contains(urlLower, "youtu.be")
	return hasYouTube
}

func extractVideoID(urlStr string) string {
	urlStr = strings.TrimSpace(urlStr)

	if strings.Contains(urlStr, "youtube.com/watch") && strings.Contains(urlStr, "v=") {
		parts := strings.Split(urlStr, "v=")
		if len(parts) > 1 {
			videoID := parts[1]
			if ampIndex := strings.Index(videoID, "&"); ampIndex != -1 {
				videoID = videoID[:ampIndex]
			}
			if hashIndex := strings.Index(videoID, "#"); hashIndex != -1 {
				videoID = videoID[:hashIndex]
			}
			return videoID
		}
	}

	if strings.Contains(urlStr, "youtu.be/") {
		parts := strings.Split(urlStr, "youtu.be/")
		if len(parts) > 1 {
			videoID := parts[1]
			if qIndex := strings.Index(videoID, "?"); qIndex != -1 {
				videoID = videoID[:qIndex]
			}
			if hashIndex := strings.Index(videoID, "#"); hashIndex != -1 {
				videoID = videoID[:hashIndex]
			}
			return videoID
		}
	}

	if strings.Contains(urlStr, "youtube.com/embed/") {
		parts := strings.Split(urlStr, "youtube.com/embed/")
		if len(parts) > 1 {
			videoID := parts[1]
			if qIndex := strings.Index(videoID, "?"); qIndex != -1 {
				videoID = videoID[:qIndex]
			}
			if hashIndex := strings.Index(videoID, "#"); hashIndex != -1 {
				videoID = videoID[:hashIndex]
			}
			return videoID
		}
	}

	return ""
}

func extractFilename(urlStr string) string {
	videoID := extractVideoID(urlStr)
	if videoID == "" {
		return fmt.Sprintf("youtube_video_%d.mp3", time.Now().Unix())
	}

	return fmt.Sprintf("%s.mp3", videoID)
}

func sanitizeFilename(filename string) string {
	forbidden := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}

	for _, char := range forbidden {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, "  ", " ")

	if len(filename) > 200 {
		ext := path.Ext(filename)
		name := strings.TrimSuffix(filename, ext)
		filename = name[:200-len(ext)] + ext
	}

	return filename
}

func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func StatusToString(status Status) string {
	switch status {
	case Pending:
		return "PENDING"
	case InProgress:
		return "DOWNLOADING"
	case Completed:
		return "COMPLETED"
	case Failed:
		return "FAILED"
	case Cancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

func getExtensionFromMimeType(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "mp4"):
		return ".mp4"
	case strings.Contains(mimeType, "webm"):
		return ".webm"
	case strings.Contains(mimeType, "m4a"):
		return ".m4a"
	case strings.Contains(mimeType, "mp3"):
		return ".mp3"
	case strings.Contains(mimeType, "audio"):
		return ".m4a"
	default:
		return ".mp4"
	}
}

func isMimeTypeAudioOnly(mimeType string) bool {
	return mimeType == "audio/mp4" || mimeType == "audio/webm" || strings.HasPrefix(mimeType, "audio/")
}

func (m *DownloadManager) escapeMetadata(value string) string {
	value = strings.ReplaceAll(value, "\"", "'")
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "=", "-")
	value = strings.ReplaceAll(value, ";", ",")

	return strings.TrimSpace(value)
}
