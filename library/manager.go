package library

import (
	"fmt"
	"kanade/metadata"
	"os"
	"path/filepath"
	"strings"

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

func (l *Library) AddSong(song Song) {
	l.Songs = append(l.Songs, song)
}

func (l *Library) RemoveSong(songPath string) {
	for i, song := range l.Songs {
		if song.Path == songPath {
			l.Songs = append(l.Songs[:i], l.Songs[i+1:]...)
			break
		}
	}
}

func (l *Library) GetSong(songPath string) *Song {
	for _, song := range l.Songs {
		if song.Path == songPath {
			return &song
		}
	}
	return nil
}

func (l *Library) ListSongs() []Song {
	return l.Songs
}

func (l *Library) Clear() {
	l.Songs = []Song{}
}

func (l *Library) Count() int {
	return len(l.Songs)
}

func (l *Library) Contains(songPath string) bool {
	for _, song := range l.Songs {
		if song.Path == songPath {
			return true
		}
	}
	return false
}

func (l *Library) UpdateSong(songPath string, updatedSong Song) {
	for i, song := range l.Songs {
		if song.Path == songPath {
			l.Songs[i] = updatedSong
			return
		}
	}
}

func (l *Library) FindByTitle(title string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Title == title {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByArtist(artist string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Artist == artist {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByAlbum(album string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Album == album {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByGenre(genre string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Genre == genre {
			results = append(results, song)
		}
	}
	return results
}

func (l *Library) FindByPath(path string) []Song {
	var results []Song
	for _, song := range l.Songs {
		if song.Path == path {
			results = append(results, song)
		}
	}
	return results
}

func ValidateFile(filePath string) error {

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file not accessible: %w", err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", filePath)
	}

	if info.Size() == 0 {
		return fmt.Errorf("file is empty: %s", filePath)
	}

	if info.Size() < 1024 {
		return fmt.Errorf("file too small to be valid audio: %s (size: %d bytes)", filePath, info.Size())
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3", ".wav":

		break
	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	return nil
}

func isSupportedAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".mp3" || ext == ".wav"
}

func (l *Library) ReadDir(dir string) ([]Song, error) {
	if dir == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("directory not accessible: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", dir)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var songs []Song
	var errors []string

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
			errors = append(errors, fmt.Sprintf("skipping %s: %v", filename, err))
			continue
		}

		meta, err := metadata.ExtractMetadata(filePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to extract metadata from %s: %v", filename, err))
			continue
		}

		song := Song{
			Path: filePath,
		}

		if meta != nil {
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

	l.Songs = append(l.Songs, songs...)

	if len(errors) > 0 && len(songs) > 0 {

		fmt.Printf("Warning: encountered %d file processing errors:\n", len(errors))
		for _, errMsg := range errors {
			fmt.Printf("  - %s\n", errMsg)
		}
	} else if len(errors) > 0 && len(songs) == 0 {

		return nil, fmt.Errorf("no valid audio files found. Errors encountered:\n%s", strings.Join(errors, "\n"))
	}

	return songs, nil
}

func (l *Library) RefreshSong(songPath string) error {

	songIndex := -1
	for i, song := range l.Songs {
		if song.Path == songPath {
			songIndex = i
			break
		}
	}

	if songIndex == -1 {
		return fmt.Errorf("song not found in library: %s", songPath)
	}

	if err := ValidateFile(songPath); err != nil {

		l.RemoveSong(songPath)
		return fmt.Errorf("song file is no longer valid, removed from library: %w", err)
	}

	meta, err := metadata.ExtractMetadata(songPath)
	if err != nil {
		return fmt.Errorf("failed to refresh metadata: %w", err)
	}

	updatedSong := l.Songs[songIndex]
	if meta != nil {
		updatedSong.Title = meta.Title()
		updatedSong.Artist = meta.Artist()
		updatedSong.Genre = meta.Genre()
		updatedSong.Album = meta.Album()
		updatedSong.Picture = meta.Picture()
	}

	l.Songs[songIndex] = updatedSong
	return nil
}
