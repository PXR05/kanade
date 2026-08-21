package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kanade/metadata"

	"github.com/dhowden/tag"
)

type Song struct {
	Title   string
	Artist  string
	Genre   string
	Album   string
	Picture *tag.Picture
	Path    string
}

type Library struct {
	Songs []Song
}

func (l *Library) ListSongs() []Song {
	return l.Songs
}

var SupportedAudioExtensions = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".ogg":  true,
}

func isSupportedAudioFile(filename string) bool {
	return SupportedAudioExtensions[strings.ToLower(filepath.Ext(filename))]
}

func ValidateFile(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not accessible: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", filePath)
	}
	if info.Size() < 1024 {
		return fmt.Errorf("file too small to be valid audio (%d bytes)", info.Size())
	}
	if !isSupportedAudioFile(filePath) {
		return fmt.Errorf("unsupported file format: %s", filepath.Ext(filePath))
	}
	return nil
}

func (l *Library) ReadDir(dir string) ([]Song, error) {
	songs, warnings, err := scanDir(dir)
	if err != nil {
		return nil, err
	}

	for _, w := range warnings {
		fmt.Printf("Warning: %s\n", w)
	}

	l.Songs = songs
	return songs, nil
}

func (l *Library) Reload(dir string) ([]Song, []string, error) {
	songs, warnings, err := scanDir(dir)
	if err != nil {
		return nil, warnings, err
	}
	l.Songs = songs
	return songs, warnings, nil
}

func scanDir(dir string) ([]Song, []string, error) {
	if dir == "" {
		return nil, nil, fmt.Errorf("directory path cannot be empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("path is not a directory: %s", dir)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var (
		songs    []Song
		warnings []string
	)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filename := file.Name()
		if strings.HasPrefix(filename, ".") || !isSupportedAudioFile(filename) {
			continue
		}

		filePath := filepath.Join(dir, filename)
		if err := ValidateFile(filePath); err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping %s: %v", filename, err))
			continue
		}

		song := Song{Path: filePath}
		if meta, err := metadata.ExtractMetadata(filePath); err == nil && meta != nil {
			song.Title = meta.Title()
			song.Artist = meta.Artist()
			song.Genre = meta.Genre()
			song.Album = meta.Album()
			song.Picture = meta.Picture()
		}

		if song.Title == "" {
			song.Title = strings.TrimSuffix(filename, filepath.Ext(filename))
		}
		if song.Artist == "" {
			song.Artist = "Unknown Artist"
		}
		if song.Album == "" {
			song.Album = "Unknown Album"
		}
		if song.Genre == "" {
			song.Genre = "Unknown Genre"
		}

		songs = append(songs, song)
	}

	return songs, warnings, nil
}
